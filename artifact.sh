#!/bin/bash -e

wget -q -O artifact_go.sh 'https://raw.githubusercontent.com/mdblp/tools/engineering/docker_build_with_github_token/artifact/artifact_go.sh'
chmod +x artifact_go.sh

. ./version.sh
./artifact_go.sh
