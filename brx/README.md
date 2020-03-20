## Buildroot - external ##

Contains makefile, config files, scripts to build `combined.cpio` using buildroot.


### Prerequisites ###

* buildroot v 2017.02
    * See buildroot documentation for its prerequisites


### Building ###
* set the env var `BUILDROOT` to point to the dir in which buildroot is installed (**not** the current dir)
* Run `make`. This will take a _very_ long time, especially the first time.
    * downloads source for compiler and all required packages
    * For each of 32, 64 bit:
        * builds compiler
        * builds all required packages
        * installs them
        * runs script to massage the fs so there will be no conflicts between 32, 64 bit
        * creates a cpio archive of this root fs
            * NOTE: to avoid running as root, buildroot stores file attributes in special files and applies them as the cpio is assembled. Scripts that change file perms are unlikely to work as expected.
    * Creates `combined.cpio`, a concatenation of the 32- and 64-bit CPIO archives.
        * Concatenation is a valid way of adding to a cpio archive.
        * If a file exists multiple times in the resulting cpio archive, only the first copy will be extracted.


#### Why 32-bit and 64-bit? ####

With the motherboard in Ichabod, we use a proprietary intel tool called `sysctl` to alter bios settings.  This tool is only available as a 32-bit executable, so the environment it runs in (the initramfs) must contain necessary 32-bit libraries and `ld`.

#### Why bother with buildroot? ####

* Copying the libs from another OS is problematic - updates to that OS could introduce additional dependencies or other breaking changes

#### Rebuilding ####

* Buildroot makes use of sentinel files (`.stamp_*`) to determine what stage has been reached for a given package.
    * It is generally safe to delete these files, though you may need to clean the affected package before it can rebuild.
* Deleting `out64/` or `out32/` won't break anything.
    * However it is **not** recommended as buildroot will have to rebuild its entire toolchain: even though this isn't cross-compilation, buildroot treats it as such.
* `out64/`, `out32/` contain more than packages/binaries that end up in the final image - gcc, binutils, linker, libs each of those depend on, etc.

### Make targets ###

* In the makefile, targets with names containing `32` affect only the 32-bit output dir `out32`.
    * Likewise for 64-bit.
* `br*-uninst`
    * deletes sentinel files so that packages will be re-installed in fakeroot
* `rm*`
    * triggers br*-uininst, deletes fakeroot and images
* See Makefile for a full list of targets.

### Configuration changes ###
* Any buildroot `make` target can be passed with `T=`
    * Due to the way make is set up, you may need to `make rm32`/`make rm64` to avoid simply being told that there's nothing to do.
* `make br64 T=menuconfig` will run buildroot's config menu, allowing you to change 64-bit options.
    * `make br32 T=menuconfig` for 32-bit.
    * Be careful! Config changes may have unexpected side effects. For example, enabling init-related options in buildroot are likely to break things because we use our own init script (../initramfs/init)

(TODO) document how to change busybox config

### Adding files ###

While buildroot has provisions for adding external files, building external packages, etc, we do not take advantage of this - in part because rebuilding with buildroot is slow, and in part because our method predated our adoption of buildroot. We instead create our own cpio which is appended to combined.cpio.

### Compression ###

The CPIO standard doesn't support concatenation of already-compressed cpio archives, but the kernel will happily decompress these as long as the compression flavors in question are supported and enabled. That said, we do not compress any part of the initramfs cpio before it is embedded in the kernel, because we get better compression if the entire kernel (including embedded initramfs) can be compressed at once. This compression is done by the kernel Makefile as one of the final steps.
