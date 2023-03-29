#! /bin/bash
set -x

if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl "https://api.github.com/repos/hortonworks/cloud-haunter/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/' | cut -c2-)
    if [ -z "$VERSION" ]; then
        echo "Failed to fetch latest version from GitHub"
        exit 1
    fi
fi

curl -LOs https://github.com/hortonworks/cloud-haunter/releases/download/v${VERSION}/cloud-haunter_${VERSION}_$(uname)_x86_64.tgz
tar -xvf cloud-haunter_${VERSION}_$(uname)_x86_64.tgz