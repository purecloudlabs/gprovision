#!/bin/sh -e

#$1 is the output dir
target=$(realpath $1)

cd $target

#apparently, if a file exists in one cpio, it can't be replaced by a file in a cpio appended later
rm -rf $target/init $target/sbin/init

#ldconfig doesn't get installed...
cp ../build/glibc-2.23/build/elf/ldconfig sbin/ldconfig
chmod +x sbin/ldconfig

cat >$target/etc/ld.so.conf <<EOF
/lib64
/usr/lib64
/usr/local/lib64
/lib32
/usr/lib32
/usr/local/lib32
/lib
/usr/lib
/usr/local/lib
EOF

#can't run ldconfig now - the files from br32 won't be visible until we're booted
