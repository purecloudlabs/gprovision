#source this file

export PATH=$(realpath ./bin):$PATH

if [[ ! -d .githooks ]]; then
  echo "must source this file from the dir it resides in"
  return
fi

(
  cd .git/hooks
  if [[ ! -L pre-commit ]]; then
    echo "setting up pre-commit hook"
    ln -s ../.githooks/pre-commit pre-commit
  fi
)

if [[ ! -d vendor/github.com ]]; then
  echo "running dep ensure"
  dep ensure
fi

echo Done.
mage -l
