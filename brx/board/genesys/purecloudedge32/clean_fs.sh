#!/bin/sh -e

#$1 is the output dir
target=$(realpath $1)

cd $target

[ -e bin/busybox ] && exit 1

# FIXME make this robust

#if [ -L lib32 ];then
  rm -fr lib32
  mv lib lib32
  mkdir lib
  ln -s ../lib32/ld-linux.so.2 lib/ld-linux.so.2
# fi
#if [ -L usr/lib32 ];then
  rm -fr usr/lib32
  mv usr/lib usr/lib32
#else
#  rm usr/lib
#fi
rm -rf bin dev etc init linuxrc media mnt opt proc root run sbin sys tmp usr/bin usr/sbin usr/share var

#fix missing libstdc++
cp -d ../build/host-gcc-final-6.3.0/build/i686-buildroot-linux-gnu/libstdc++-v3/src/.libs/libstdc++.so* lib32
