go-ircd
=======

An ircd written in Go

contributing
------------

to contribute, *please* install the git hook to check your code.

this can be done by running `ln -s ../../pre-commit.sh .git/hooks/pre-commit` in the root of the repo.

after that, the [pre-commit.sh](pre-commit.sh) script will be run before every commit. if any command fails, the commit fails and you must fix the problems.

to use the hook, you will need the vet tool, which does not ship with go as of this writing. 

to get vet, run `go get code.google.com/p/go.tools/cmd/vet`. this will install vet in $GOROOT, so make sure you can write there or have root.

