#source this file

export GOPATH=$(realpath ./gopath)
export PATH=$(realpath ./bin):$PATH

(
  cd .git/hooks
  if [[ ! -e pre-commit ]]; then
    echo "setting up pre-commit hook"
    ln -s ../.githooks/pre-commit pre-commit
  fi
)

(
  cd ./gopath/src/gprovision
  if [[ ! -d vendor/github.com ]]; then
    echo "run dep ensure"
    dep ensure
  fi
)

echo available targets:
mage -l
