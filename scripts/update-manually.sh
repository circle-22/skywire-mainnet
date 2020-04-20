#!/usr/bin/env bash

OS="linux"
ARCH="arm"
VERSION=v0.2.2-test-ba0b6a42
BASE_NAME="skywire-${VERSION}-${OS}-${ARCH}"
FOLDER_NAME="${BASE_NAME}"
ARCHIVE_NAME="${BASE_NAME}.tar.gz"
SKYWIRE_URL="https://github.com/SkycoinProject/skywire-mainnet/releases/download/${VERSION}/${ARCHIVE_NAME}"
VISOR_NAME="skywire-visor"

rm -f "./${ARCHIVE_NAME}"
wget -O $ARCHIVE_NAME "$SKYWIRE_URL"
rm -rf "./${FOLDER_NAME}"

mkdir "./${FOLDER_NAME}"
tar -xf "./${ARCHIVE_NAME}" -C "./${FOLDER_NAME}"

rm -f "./${VISOR_NAME}"
cp "./${FOLDER_NAME}/${VISOR_NAME}" "./${VISOR_NAME}"

rm -rf "./${FOLDER_NAME}"
rm -f "./${ARCHIVE_NAME}"

reboot