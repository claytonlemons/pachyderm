package pfsload

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"strconv"
	"strings"

	"github.com/pachyderm/pachyderm/v2/src/client"
	"github.com/pachyderm/pachyderm/v2/src/internal/errors"
	"github.com/pachyderm/pachyderm/v2/src/internal/storage/fileset"
	"github.com/pachyderm/pachyderm/v2/src/pfs"
)

type Validator struct {
	spec   *ValidatorSpec
	random *rand.Rand
	hash   []byte
	count  int64
}

func NewValidator(spec *ValidatorSpec, random *rand.Rand) (*Validator, error) {
	if spec.Frequency != nil {
		if err := validateProb(int(spec.Frequency.Prob)); err != nil {
			return nil, err
		}
	}
	return &Validator{
		spec:   spec,
		random: random,
		hash:   pfs.NewHash().Sum(nil),
	}, nil
}

func (v *Validator) AddHash(hash []byte) {
	v.hash = xor(v.hash, hash)
}

func xor(b1, b2 []byte) []byte {
	result := make([]byte, len(b1))
	for i, b := range b1 {
		result[i] = b ^ b2[i]
	}
	return result
}

// TODO: Distribute validation and revert back to being based on file path + content rather than
// file path + size.
func (v *Validator) Validate(client Client, commit *pfs.Commit) (retErr error) {
	if v.spec.Frequency != nil {
		freq := v.spec.Frequency
		if freq.Count > 0 {
			v.count++
			if freq.Count > v.count {
				return nil
			}
			v.count = 0
		} else if !shouldExecute(v.random, int(freq.Prob)) {
			return nil
		}
	}
	err := client.WaitCommitSet(commit.ID, func(ci *pfs.CommitInfo) error {
		if ci.Commit.Branch.Repo.Type != pfs.UserRepoType {
			return nil
		}
		return validate(client, ci.Commit, v.hash)
	})
	return errors.EnsureStack(err)
}

func validate(client Client, commit *pfs.Commit, expectedHash []byte) error {
	hash := pfs.NewHash().Sum(nil)
	if err := client.GlobFile(client.Ctx(), commit, "**", func(fi *pfs.FileInfo) error {
		if strings.HasSuffix(fi.File.Path, "/") {
			return nil
		}
		h := pfs.NewHash()
		h.Write(append([]byte(fi.File.Path), []byte(strconv.FormatInt(fi.SizeBytes, 10))...))
		hash = xor(hash, h.Sum(nil))
		return nil
	}); err != nil {
		return errors.EnsureStack(err)
	}
	if !bytes.Equal(hash, expectedHash) {
		return errors.Errorf("hash of files at commit %v different from validator hash", commit)
	}
	return nil
}

type validatorClient struct {
	Client
	hash []byte
}

func NewValidatorClient(client Client) *validatorClient {
	return &validatorClient{
		Client: client,
	}
}

func (vc *validatorClient) WithCreateFileSetClient(ctx context.Context, cb func(client.ModifyFile) error) (*pfs.CreateFileSetResponse, error) {
	resp, err := vc.Client.WithCreateFileSetClient(ctx, func(mf client.ModifyFile) error {
		vcfsc := &validatorCreateFileSetClient{
			ModifyFile: mf,
		}
		if err := cb(vcfsc); err != nil {
			return err
		}
		if vcfsc.hash == nil {
			return nil
		}
		if vc.hash == nil {
			vc.hash = vcfsc.hash
			return nil
		}
		vc.hash = xor(vc.hash, vcfsc.hash)
		return nil
	})
	return resp, errors.EnsureStack(err)
}

type validatorCreateFileSetClient struct {
	client.ModifyFile
	hash []byte
}

func (vcfsc *validatorCreateFileSetClient) PutFile(path string, r io.Reader, opts ...client.PutFileOption) error {
	path = fileset.Clean(path, false)
	sr := &sizeReader{Reader: r}
	if err := vcfsc.ModifyFile.PutFile(path, sr, opts...); err != nil {
		return errors.EnsureStack(err)
	}
	hash := pfs.NewHash()
	hash.Write(append([]byte(path), []byte(strconv.FormatInt(sr.size, 10))...))
	if vcfsc.hash == nil {
		vcfsc.hash = hash.Sum(nil)
		return nil
	}
	vcfsc.hash = xor(vcfsc.hash, hash.Sum(nil))
	return nil
}

type sizeReader struct {
	io.Reader
	size int64
}

func (sr *sizeReader) Read(data []byte) (int, error) {
	n, err := sr.Reader.Read(data)
	sr.size += int64(n)
	return n, errors.EnsureStack(err)
}
