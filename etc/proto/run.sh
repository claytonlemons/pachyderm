#!/bin/bash
set -ex

export GOGO_PROTO_VERSION="v1.3.2"

tar -C "${GOPATH}/src/github.com/pachyderm/pachyderm" -xf /dev/stdin

# Make sure the proto compiler is reasonably up-to-date so that our compiled
# protobufs aren't generated by stale tools
max_age="$(date --date="1 month ago" +%s)"
if [[ "${max_age}" -gt "$(cat /last_run_time)" ]]; then
    PRE="\e[1;31m~\e[0m"
    echo -e "\e[1;31m~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~\e[0m" >/dev/stderr
    echo -e "${PRE} \e[1;31mWARNING:\e[0m pachyderm_proto is out of date" >/dev/stderr
    echo -e "${PRE} please run" >/dev/stderr
    echo -e "${PRE}   make DOCKER_BUILD_FLAGS=--no-cache proto" >/dev/stderr
    echo -e "\e[1;31m~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~\e[0m" >/dev/stderr
    exit 1
fi

cd "${GOPATH}/src/github.com/pachyderm/pachyderm"

mkdir -p v2/src

# shellcheck disable=SC2044
for i in $(find src -name "*.proto"); do \
    if ! grep -q 'go_package' "${i}"; then
        echo -e "\e[1;31mError:\e[0m missing \"go_package\" declaration in ${i}" >/dev/stderr
    fi
    protoc \
        "-I${GOPATH}/pkg/mod/github.com/gogo/protobuf@${GOGO_PROTO_VERSION}" \
        -Isrc \
        --gogofast_out=plugins=grpc,\
Mgoogle/protobuf/duration.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/empty.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types,\
Mgoogle/protobuf/wrappers.proto=github.com/gogo/protobuf/types,\
Mgogoproto/gogo.proto=github.com/gogo/protobuf/gogoproto,\
Mgoogle/protobuf/any.proto=github.com/gogo/protobuf/types,\
":${GOPATH}/src" \
    "${i}" >/dev/stderr
done

pushd src > /dev/stderr
read -ra proto_files < <(find . -name "*.proto" | sort | xargs)
protoc \
    --proto_path . \
    --plugin=protoc-gen-pach="${GOPATH}/bin/protoc-gen-pach" \
    "-I${GOPATH}/pkg/mod/github.com/gogo/protobuf@${GOGO_PROTO_VERSION}" \
    --pach_out="../v2/src" \
    "${proto_files[@]}" > /dev/stderr

popd > /dev/stderr

# TODO (brendon): figure out how to configure protoc
pushd v2 > /dev/stderr
gofmt -w src > /dev/stderr
find src -regex ".*\.go" -print0 | xargs -0 tar cf -
