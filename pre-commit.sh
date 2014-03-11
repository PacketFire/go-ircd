#!/bin/sh
#
# pre-commit.sh
#
# fmt and test everything. check for garbage being comitted.
# stash changes so only staged code is tested.
#
# add a check for bullshit like 'panic()' being comitted
#

# stash only if we have unstaged changes, pop at exit if so
if ! git diff-files --quiet
then
    # There are unstaged changes; stash and then pop when script exits
    git stash -q --keep-index
    trap 'git stash pop -q' EXIT 
fi

go build ./... >/dev/null 2>&1
if [ $? -ne 0 ]
then
  echo "Failed to build project. Please check the output of"
  echo "go build or run commit with --no-verify if you know"
  echo "what you are doing."

  exit 1
fi

go test ./... >/dev/null 2>&1
if [ $? -ne 0 ]
then
  echo "Failed to run tests. Please check the output of"
  echo "go test or run commit with --no-verify if you know"
  echo "what you are doing."

  exit 1
fi

go fmt ./... >/dev/null 2>&1
if [ $? -ne 0 ]
then
  echo "Failed to run go fmt. This shouldn't happen. Please"
  echo "check the output of the command to see what's wrong"
  echo "or run commit with --no-verify if you know what you"
  echo "are doing."

  exit 1
fi

go vet ./... >/dev/null 2>&1
if [ $? -ne 0 ]
then
  echo "go vet has detected potential issues in your project."
  echo "Please check its output or run commit with --no-verify"
  echo "if you know what you are doing."

  exit 1
fi

echo "ALL OK"

