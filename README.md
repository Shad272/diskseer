# diskseer

**Disk diagnostics that gives you a verdict, not a spreadsheet.**

*Output is in Italian: the tool is used with Italian customers. The code and
this document are in English.*

Every diagnostic tool shows you the same thing: thirty SMART attributes, a
temperature, a wear counter. Then it leaves you alone with them.

`diskseer` runs one command and tells you what is actually wrong and what to do
about it, in plain language.

```
  ESITO: CRITICO  1 problema grave, 2 da tenere d'occhio

  ● CRITICO  Spazio · C:
  Solo 2.4 GB liberi su 476 GB (0.5%)
  Lo spazio libero è praticamente esaurito. A questo livello il file di
  paging non può crescere, gli aggiornamenti falliscono e alcuni programmi
  si chiudono senza spiegazione.
  → Liberare spazio subito, prima di qualsiasi altra diagnosi: molti
    sintomi spariranno da soli.
```

## Why

I repair PCs. The single most common job is "my computer is slow", and the
answer is almost always one of five things: a mechanical system disk, a full
system volume, thermal throttling, a dying drive, or a worn battery.

Existing tools have all the data needed to say this. None of them say it.
`diskseer` is the missing layer: a rules engine that turns measurements into a
diagnosis a person can act on.

## Build

Single binary, no dependencies outside the standard library. Windows only for
now.

```
go build -o diskseer.exe .
```

## Usage

```
diskseer              # full report
diskseer --json       # raw data, for scripting or archiving
diskseer --no-color   # plain text

# Strip machine identity before sharing a capture
diskseer --json --anonimo > case.json

# Client-ready report as a self-contained HTML page
diskseer --html report.html \
      --tecnico "Your Name" --contatto "you@example.com" --cliente "Customer"
```

Run it **as administrator** for the complete picture. Without elevation
Windows hides SMART counters, drive temperatures and wear levels, and
`diskseer` will tell you so rather than pretending the disk is healthy.

Exit codes: `0` clean, `1` warnings, `2` critical, `3` collection failed.
Useful for sweeping a fleet and surfacing only the machines that need work.

## What it checks

| Area | Checks |
|---|---|
| Storage | uncorrected read/write errors, NVMe media errors, life used and projected remaining life, spare capacity, current and peak temperature, thermal throttling minutes, power-on hours, start/stop cycles |
| Performance | OS installed on a mechanical drive, aggressive head parking |
| Space | free space on every volume, file system repair level (scan / spot fix / full repair) |
| Thermal | ACPI thermal zones, measured minutes spent above the drive throttle threshold |
| Battery | capacity against design, charge cycles |

## How it works

Three layers, deliberately separated:

- **`internal/collect`** — reads raw data from the machine. Windows via a
  single embedded PowerShell probe; one file per OS.
- **`internal/rules`** — turns a snapshot into findings. Every rule is an
  independent function. This is where the actual knowledge lives.
- **`internal/report`** — renders findings for humans.

Adding Linux support means writing one collector. The diagnostic logic is
shared and untouched.

## Design notes

**Missing data is not zero.** Every value the OS may fail to provide is a
pointer. A drive with zero errors is healthy; a drive whose error count we
could not read is unknown. Conflating the two is how diagnostic tools lie.

**No `-EncodedCommand`.** The probe is written to a temp file and run with
`-File`. Base64-encoded PowerShell is a well-known malware signature and
trips antivirus and EDR. A diagnostic tool that sets off the customer's
antivirus never gets opened twice.

**Thresholds are the product.** They live in one place per area and are meant
to be corrected when a diagnosis turns out wrong in the field.


## How NVMe health is read

Windows exposes reliability counters through `Get-StorageReliabilityCounter`,
but for NVMe drives it returns **neither power-on hours nor error counters** —
the two numbers that tell you whether a drive is dying.

So `diskseer` asks the drive itself. It opens `\.\PhysicalDriveN` and issues
`IOCTL_STORAGE_QUERY_PROPERTY` with an NVMe protocol-specific payload to fetch
log page 0x02, the SMART / Health Information page, then parses the 512 bytes
by offset. That yields media errors, unsafe shutdowns, spare capacity, life
used, throttling minutes and the drive's own critical-warning flags.

It tries opening the device with **zero** requested access rights first, and
only falls back to `GENERIC_READ | GENERIC_WRITE` if that fails. In practice
the first attempt succeeds, so NVMe health is available **without
administrator privileges** — unlike everything Windows mediates.

## How SATA and USB S.M.A.R.T. are read

Same idea as NVMe — talk to the drive, not to Windows — but a much older
protocol. Where NVMe returns a log page with fixed offsets defined by the
standard, ATA drives work by filling controller registers: you write the
command into the registers, the drive answers with 512 bytes, and inside are
thirty numbered attributes whose numbering is standard only by convention.

`diskseer` issues `SMART_RCV_DRIVE_DATA` with `SENDCMDINPARAMS` carrying the
ATA `SMART READ ATTRIBUTES` command (0xB0 / 0xD0), checks the driver and IDE
status bytes in the reply before trusting a single byte of it, and parses the
attribute table.

The payoff is precision Windows cannot offer. Windows says "uncorrected
errors". The drive distinguishes:

| Attribute | Meaning | Why it matters |
|---|---|---|
| 5 | sectors already replaced | deterioration started |
| 197 | sectors unreadable **right now** | data currently inaccessible |
| 198 | unreadable and given up on | data permanently lost |
| 199 | transfer errors on the cable | **the drive is fine — the cable isn't** |
| 10 | failed spin-up attempts | motor failing, sudden total loss ahead |
| 193 | head park cycles | mechanical wear budget |

Attribute 199 is the one that pays for the whole feature: bad cables produce
symptoms identical to a dying drive, and healthy drives get replaced over it
every day. A €3 cable fixes it.

Unlike NVMe, this **does** require administrator privileges: the drive accepts
the command only on a handle opened with `GENERIC_READ | GENERIC_WRITE`, and
Windows grants that on `\.\PhysicalDriveN` only to administrators. `diskseer`
still attempts the zero-access path first and falls back, so it takes the
lowest privilege that works.

## Limitations

Counter availability by bus, measured on real hardware:

| | SATA HDD | NVMe SSD | SATA SSD in USB enclosure |
|---|---|---|---|
| Access path | ATA IOCTL | NVMe log page | SCSI-ATA translation |
| Elevation required | yes | **no** | yes |
| Temperature | yes | yes | yes |
| Power-on hours | yes | yes | yes |
| Reallocated / pending sectors | yes | — | yes |
| Cable transfer errors | yes | — | yes |
| Unexpected power loss | — | yes | yes |
| Life used / spare | — | yes | vendor-specific |
| Throttling time | — | yes | — |

All three paths read the drive itself rather than Windows' interpretation of
it, and all three are verified against real hardware: a 5400rpm mechanical
drive over SATA, a consumer NVMe SSD, and a SATA SSD behind an ASMedia USB
bridge.

USB support depends on the bridge implementing SCSI-ATA translation. Many
cheap enclosures do not, and some accept the command and return a block of
zeroes rather than admitting it — so the reply is validated before it is
trusted. Both the 12-byte and 16-byte command formats are attempted, since
neither is universal.

Vendor-specific SSD attributes above ID 200 are deliberately left unnamed
unless their meaning is consistent across manufacturers. A wrong label is
worse than no label: an unnamed number gets looked up, a mislabelled one gets
believed.

Windows also reports `0` for counters a driver does not provide, with no way
to distinguish that from a real zero. `internal/collect.Normalize` throws away
the cases where a zero is provably meaningless (wear on a mechanical drive, a
peak temperature of 0 °C) and leaves the rest alone. Guessing further would be
worse than admitting the gap.

## Test data

The snapshots in `testdata/` are real captures, run through `--anonimo`:
manufacturer, model, CPU and drive names are replaced and timestamps zeroed,
while every measurement is left untouched. `tools/capture.ps1` anonymises by
default — captures taken on a customer's machine describe their computer, not
the fault being studied, and that data has no business in a repository.

The whole suite runs against those anonymised fixtures and passes, which is
the practical proof that anonymising changes no measurement.

## Status

Early. Windows only. Rules are being tuned against real machines.

## License

MIT
