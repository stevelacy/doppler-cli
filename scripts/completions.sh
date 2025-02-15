#!/bin/sh
set -e

rm -rf completions
mkdir completions

for sh in bash zsh; do
	go run main.go completion "$sh" > "completions/doppler.$sh"
done
