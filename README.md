# gprovision

Source code related to factory restore and provisioning (aka manufacture or imaging). Also includes buildroot files.

## target architecture

x86 linux

This has only been tested on x86, but there shouldn't be much that is x86-specific. Buildroot is used to get a repeatable build of AMD64 utilities, as well as x86-32 libraries (one tool we use is only available in 32-bit versions).

### known architecture-specific bits

A few ioctl's are used by their raw number, rather than by symbol. These numbers are unlikely to be portable. All uses should be in packages under `hw/`.

The type of the device (aka platform, variant) is currently determined from dmi/smbios values.

PCI is ubiquitous on x86 but not as common elsewhere. There is some pci-specific logic but probably not in any critical paths.


## packages that may be of particular interest

* provisioning, aka manufacture `gprovision/pkg/mfg`
* factory restore `gprovision/pkg/recovery`
* data erase `gprovision/pkg/erase`
* linux /init binary `gprovision/cmd/init`, `gprovision/pkg/init`
  * determines whether to erase, factory restore, or do normal boot
  * or, with the appropriate build tag, provisions the unit
* integ tests for the above
* crystalfontz lcd code, including menus `gprovision/pkg/hw/cfa`
* partitioning/formatting/bootability code for uefi and legacy `gprovision/pkg/recovery/disk`
* flexible logging to multiple sinks (lcd, file, console) `gprovision/pkg/log`
* appliance package identifies the hardware variant it's running on `gprovision/pkg/appliance`
* config of ipmi account, password `gprovision/pkg/hw/ipmi`

## proprietary/oss

Some packages couldn't be open sourced, and a few provided functionality that is absolutely required; for those, replacements were written and live under `gprovision/pkg/oss`. These packages are related to remote logging, record keeping, and factory restore data persistence.

To facilitate open source release while still using proprietary packages within genesys, `gprovision/pkg/common` (and subpackages) was added. This uses interfaces to abstract away exactly what package is providing the functionality. In a few dirs, you will find an `oss.go` file; internally, we use a `proprietary.go` in place of `oss.go`, to similar effect. These files have the effect of initializing interface vars in the aforementioned common package.

If you wish to customize gprovision's behavior, start by looking at the interfaces in `common` - re-implementing one or more of these may provide all you need.

## not included

### security

The code we could open source does *not* include a mechanism for secure storage of passwords. Password storage is required for the integ tests, so some sort of implementation was required. To discourage use of the simplistic and weak mechanism included in this code, it deliberately uses annoying passwords and emits warnings.

### image creation

This code does not include anything to build the images (`*.upd`).

### OTA

A mechanism for distribution of over-the-air updates is not included, nor is any sort of fleet status/control dashboard. This code does not (currently) integrate with mender.io but that's probably the most logical choice for those functionalities.

## image

### name

The image name must match a specific format: `WIDGET.LNX.SHINY.YYYY-MM-DD.NNNN.upd`
* extension: `.upd`
* prefix must match the string returned by common/strs.ImgPrefix()
  * default is `WIDGET.LNX.SHINY.`
    * product: widget
    * os: lnx
    * hardware platform: shiny
* remaining fields are date and a build number
  * see recovery/archive package for additional info including sort algo

### format

* xz-compressed tarball
  * xz _must_ be invoked with the sha256 checksum option
  * treated as invalid if:
    * xz signature is missing
    * xz checksum type is not sha256
    * xz checksum validation fails

### content

* a tarball of the root filesystem that will be laid down
  * assumptions
    * systemd is the init system
    * systemd-networkd manages networking
    * `/boot/norm_boot` is the kernel built by mage in this repo, with embedded initramfs
* on install, the kernel located under /boot in the image is compared to the previously installed kernel
  * comparison is by _build number_, not kernel version
    * example: `4.19.16 (user@host) #300 SMP <timestamp>` => build number is 300
    * use the kernel built here by mage
      * build number is set from an env var
    * kernel with higher number will overwrite the other
  * do not include any boot-related files except the kernel; they will be ignored.
    * grub
    * grub menus
    * separate initramfs
    * etc
* accounts
  * admin must exist
  * no accounts in the image (other than admin) should allow login
  * factory restore will set password for admin account

### file location

Factory restore looks for update files on the recovery volume, under Image/. The first image is written during provisioning. Others are the responsibility of your code to download/place. Note that factory restore verifies the checksum to avoid corruption, but this does not provide any guarantees that the image came from the right place.

Your download code should have additional safeguards, such as

* signature verification
* only allow downloads from a host under your control
* only allow https downloads
* pin the certificate for the download host
* etc

## building

### GOPATH, PATH
With bash on linux, run
    . ./bash_env.sh
to set up your env. This also runs `dep ensure` if it looks like you haven't, and lists mage targets.

For other shells, you'll need to set GOPATH and update PATH to include the repo's `/bin` so that our `mage` is found before any version you may have installed.

### TEMPDIR

For integ tests, your /tmp must have plenty of free space. If it has < 8GB or so, set the TEMPDIR env var to point to a dir that does have that much free space.

Passing tests delete their temp dirs, but failing tests generally do not. Re-running a failed integ test will delete temp dirs from previous runs of that test. This delete is done with a  wildcard like `test-gprov-erase*`, so there is a slight but non-zero chance it would delete something it didn't create.

### mage
Using mage is not necessary for simple packages like the crystalfontz code, but it is needed for things like the kernels with embedded initramfs and for integ tests. These integ tests depend on env vars mage sets, and are skipped if the vars are not set.

Due to required dependencies, mage will not run until after you run `dep` - see its section below.

Mage targets (listed with `mage -l`) are case insensitive, and each corresponds to a public function in $GOPATH/src/mage. For example, `mage bins:embedded` runs the function with signature `func (Bins) Embedded(ctx context.Context)`.

### qemu
qemu is also needed for integ tests. See `qemu/` in the repo root for a Dockerfile that'll build qemu and copy the necessary files out. A pre-built version is available on github under releases, and is automatically downloaded by mage.

### dep
dep is needed. Once installed, `cd ./gopath/src/gprovision` and run `dep ensure` to download all dependencies.

### protoc
protoc, the protocol buffer compiler, is needed for code generation.

### kernel
All tools required to build the kernel are needed - gcc, flex, bison, make, binutils, etc. See kernel documentation for details.

#### kernel version
The kernel source to download is determined from the name of the `linux-*.config` file.

### buildroot
Files in `brx/` are inputs to build a cpio of non-go binaries. A pre-built version is available on github under releases, and is automatically downloaded by mage.

### Known to work with...
Known to work with `go v1.12.5` or higher, the latest `dep`, and `protoc 3.x`.

### go packages
To pull in dependencies:

    cd $GOPATH/src/gprovision
    dep ensure

### verify
Once that is done, running `mage -l` from any dir should print a list of targets.

`mage tests:integ`, for example, will build prerequisites and then run integ tests.

## Known issues

The integ tests are occasionally flaky, as are the crystalfontz tests.

Shells out too much. u-root provides many packages we could use to avoid shelling out, but don't (yet).

## Sequence

### Provision, AKA manufacture

    +---------+     +---------+
    | Legacy  |     | UEFI    |
    | PXEboot |     | PXEboot |
    +----+----+     +----+----+
         |               |
         +-------+-------+
                 |
                 v
         +-------+-------+
         | provision.pxe |
         +-------+-------+
                 |
                 v
       +---------+----------+
       | load json from url |
       |  passed in kernel  |
       | parameter 'mfgurl' |
       +---------+----------+
                 |
                 v
     +-----------+-------------+
     | verify hardware specs,  |
     | download image & write  |
     |   to recovery volume,   |
     | write credentials, etc  |
     +-----------+-------------+
                 |
                 v
        +--------+---------+
        |  configure boot  |
        | (legacy or uefi) |
        +--------+---------+
                 |
                 v
            +----+-----+
            |  reboot  |
            +----------+

See Infrastructure below for off-device services that are required.

### All boots after manufacture

      +--------+   +------+
      | Legacy |   | UEFI |
      |  boot  |   | boot |
      +----+---+   +---+--+
           |           |
           +-----+-----+
                 |
                 v
        +--------+--------+
        | "normal" kernel |
        +--------+--------+
                 |
                 v
         +-------+-------+
         | erase needed? |
         +--+----------+-+
            |          |
            v          v
          +-+--+    +--+--+
          | no |    | yes |
          +-+--+    +--+--+
            |          |
            |          v
            |      +---+---+     +--------+
            |      | erase +---->+ reboot |
            |      +-------+     +--------+
            |
            v
     +------+---------------+
     | locate primary drive |
     +----------+-----------+
                |
                v
    +-----------+-----------+
    | does flag file exist? |
    +--+--------------+-----+
       |              |
       v              v
    +--+-+         +--+--+
    | no |         | yes |
    +--+-+         +--+--+
       |              |
       |              v
       |       +------+--------+
       |       | all good,     |     +----------+
       |       | start systemd +---->+ eventual |
       |       | from primary  |     |  reboot  |
       |       +---------------+     +----------+
       v
     +-+---------------------+
     | start factory restore |
     +----------+------------+
                |
                v
     +----------+-------------------------+
     |                                    |       +--------+
     | find latest good image on recovery +------>+ reboot |
     | format primary & write image       |       +--------+
     | write network config               |
     | write flag file                    |
     | etc                                |
     |                                    |
     +------------------------------------+


## Infrastructure for provisioning/manufacture

The integ tests (especially lifecycle and mfg) set up minimal infrastructure, which could be used as examples for some of the infrastructure.

For production, you will need the following:

* DHCP and TFTP set up for PXEboot
  * Setting up a network that will PXEboot both legacy and uefi can be painful. If you can get away with uefi-only, great!
  * iPXE supports http transfers. They say this is far faster than tftp, so serve only the bare minimum (iPXE) from TFTP and everything else over http (assuming you use iPXE)
* web server
  * serve files needed by iPXE (if used) - menus, provisioning kernel
  * serve file passed to kernel as mfgurl
    * see below
* log server
  * collects logs produced by devices being imaged
  * included implementation (cmd/util/pbLogServer, pkg/oss/pblog/server) bundles additional functionality
    * password generation & storage
    * qa doc printing
    * other record keeping
    * web server for viewing logs

### mfgurl / manufData.json

* format: json
* lists additional files needed, common to all hardware variants
* specific to a particular variant:
  * hardware characteristics for validation (# cpus, pci devices present, etc)
  * additial configuration steps

As an example, see [doc/manufDataSample.json](doc/manufDataSample.json).

Exactly what variant a device is, is determined by the [appliance](gopath/src/gprovision/pkg/appliance) package. That package primarily uses dmi/smbios fields.
