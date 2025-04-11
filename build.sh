#!/usr/bin/env bash

rm build/*

BINNAME=${PWD##*/}
echo Building $BINNAME

go build -o ./build/default
chmod +x ./build/default
BINVER=$(./build/default --version)
rm ./build/default
echo Version $BINVER

ARCHITECTURES=(
  "windows amd64 .exe" 
  "linux amd64"
  "linux arm64"
)

for ARCHITECTURE in "${ARCHITECTURES[@]}"
do
  read BINOS BINARCH BINEXT <<< "$ARCHITECTURE"
  echo "Building ${BINNAME}_${BINVER}_${BINOS}_$BINARCH$BINEXT"
  GOOS=$BINOS GOARCH=$BINARCH go build -o "./build/${BINNAME}_${BINVER}_${BINOS}_$BINARCH$BINEXT" .
done

echo Done