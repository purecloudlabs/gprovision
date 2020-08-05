# grub4dos

## To rebuild

Clone grub4dos. Known to work with rev 3c1d05f39.

Overwrite build script with our version. IIRC the differences are to skip binaries we don't use; you _should_ be able to skip this step if desired.

Run build script.

### In script form

    git clone github.com/chenall/grub4dos
    cd grub4dos
    git checkout 3c1d05f39e49ec1d7543caa825df00068b96620b
    cp ../build .
    ./build
