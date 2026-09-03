# diskseer

**Disk diagnostics that gives you a verdict, not a spreadsheet.**

Every SMART tool shows you the same thing: thirty attributes, a temperature, a
wear counter. Then it leaves you alone with them.

`diskseer` runs one command and tells you what is actually wrong, why it
knows, and what to do about it.

```
  ──────────────────────────────────────────────────────────────────────────
  RESULT: CRITICAL  1 serious problem, 2 things to keep an eye on
  ──────────────────────────────────────────────────────────────────────────

  ● CRITICAL  Drive · SATA HDD #0
  3 unreadable sectors awaiting reallocation
  The drive found sectors it can no longer read and is still trying to
  recover them. While they stay in this state the data they held is
  inaccessible: if files lived there, those files are already damaged.
  This counter is the signal that precedes the failure of a mechanical
  drive, and it appears long before the user notices anything.
  → Copy the data today, starting with whatever is irreplaceable. Do not
    defragment and do not run full scans: they add stress to a drive that
    is already failing.

  ● WARNING  Connection · SATA HDD #0
  14 cable transfer errors
  Data was corrupted in transit between the drive and the motherboard,
  not inside the drive. The cause is almost always the cable or the
  connector. The symptoms — freezes, corrupted files, the drive vanishing
  from the system — are identical to a dying drive, which is why
  perfectly healthy drives get replaced over this.
  → Replace the data cable and check it is firmly seated at both ends,
    before considering replacing the drive.
```

Single binary. No installer, no dependencies, no telemetry. Windows.

**[Download the latest release →](../../releases/latest)**

---

## Why this exists

I repair computers. The most common job by far is "my PC is slow", and the
answer is almost always one of five things: the OS on a mechanical drive, a
full system volume, thermal throttling, a dying disk, or a bad SATA cable.

Every one of those is visible in data the machine already exposes. No tool
says it out loud. They print the numbers and leave the interpretation to you —
which is fine if you already know, and useless if you don't.

`diskseer` is the missing layer: a rules engine that turns measurements into a
diagnosis someone can act on, plus a report you can hand to a customer.

## What makes it different

**It reads the drive, not Windows' opinion of the drive.** Three access paths,
one per drive family, all implemented from scratch against the raw Windows
APIs — no external tools shelled out to, no dependencies.

**It says what to do.** Every finding carries an action. A finding without one
is just a number with extra words.

**It admits what it could not check.** Run without administrator rights and it
names the drives it could not reach, in the results box, before the findings —
because knowing half the check was skipped changes the weight of everything
that follows.

**It never confuses "unknown" with "zero".** Windows reports `0` for counters a
driver does not provide, with no way to tell that apart from a genuine zero.
Treating those at face value is how diagnostic tools declare a dying drive
healthy.

## Diagnoses other tools do not give you

| | What it catches | Why it matters |
|---|---|---|
| **Bad SATA cable** | transfer errors (attr. 199) | symptoms are identical to a failing drive; healthy drives get replaced over this every day. A €3 cable fixes it |
| **Aggressive head parking** | park cycles vs powered hours | burns the drive's mechanical budget years early while passing every health check |
| **Historic thermal throttling** | peak temperature, minutes above threshold | the drive is cool when you test it; it was not when the user complained |
| **Remaining life, not life used** | wear % projected over powered hours | "60% used" means nothing alone. 60% in ten years is fine. 60% in eight months means it dies next year |
| **Unsafe shutdown → corrupted volume** | drive counters cross-referenced with volume state | links a cause on one disk to a symptom on another |
| **Which repair a volume needs** | `OperationalStatus`, not just health | a spot fix takes ten seconds; a full repair needs your data backed up first |

That fifth one is the tool's signature move. It reads a counter *inside* one
drive and connects it to a corrupted file system on a *different* one:

```
  ● WARNING  System · NVMe SSD #1
  32 unsafe shutdowns out of 779 power-ups
  Out of 779 power-ups, 32 times the computer shut down while the drive
  was still writing (4.1%): power loss, freezes, or holding the power
  button. The drive survives it undamaged, but this is exactly how file
  systems get corrupted. On this machine the following indeed needs
  repair: E:.
  → Find the cause of the shutdowns: power supply, overheating, system
    freezes. Repairing file systems without removing the cause means
    doing it again and again.
```

That is the reasoning a good technician does in their head. No diagnostic tool
does it for them.

## Install

Download `diskseer.exe` from [Releases](../../releases/latest) and run it.
Nothing to install.

Windows will show *"Windows protected your PC"* because the binary is not code
signed — click **More info → Run anyway**. Certificates cost a few hundred
euros a year and this project has no budget for one.

Or build it yourself:

```
git clone https://github.com/Shad272/diskseer.git
cd diskseer
go build -o diskseer.exe .
```

Go 1.21 or newer. No module dependencies — check `go.mod`, it is four lines.

## Usage

Double-click it and it does the right thing: asks for administrator rights,
prints the report, saves an HTML copy next to itself, and waits before closing.

From a terminal:

```
diskseer                     # full report
diskseer --lang it           # report in Italian
diskseer --json              # raw data, for scripts
diskseer --json --anonymous  # raw data with the machine identity stripped
```

Hand a customer a document:

```
diskseer --html report.html --technician "Your Name" ^
         --contact "you@example.com" --customer "Their Name"
```

The HTML is self-contained: no external stylesheet, no remote fonts, no
tracking. It opens on a machine with no internet — which is often the machine
you are diagnosing — and prints to PDF without breaking cards across pages.

**Run it as administrator** for the full picture. Without elevation, Windows
hides SATA and USB drive health, and `diskseer` will tell you so rather than
pretending the disk is fine.

Exit codes: `0` clean, `1` warnings, `2` critical, `3` collection failed —
useful for sweeping a fleet and surfacing only the machines that need work.

---

## How it reads the drives

Three drive families, three completely different protocols. All three are
verified against real hardware.

### NVMe — log page 0x02

Windows exposes reliability counters through `Get-StorageReliabilityCounter`,
but for NVMe drives it returns **neither power-on hours nor error counters** —
the two numbers that tell you whether a drive is dying.

So `diskseer` asks the drive. It opens `\\.\PhysicalDriveN` and issues
`IOCTL_STORAGE_QUERY_PROPERTY` with an NVMe protocol-specific payload to fetch
the SMART / Health Information log page, then parses its 512 bytes by offset.
That yields media errors, unsafe shutdowns, spare capacity, life used,
throttling minutes and the drive's own critical-warning flags.

It tries opening the device with **zero** requested access rights first, and
only falls back to `GENERIC_READ | GENERIC_WRITE` if that fails. The first
attempt usually succeeds — so NVMe health needs **no administrator rights**,
unlike everything Windows mediates.

### SATA — ATA `SMART READ ATTRIBUTES`

Same idea, much older protocol. Where NVMe returns fixed offsets defined by the
standard, ATA drives work by filling controller registers: you write the
command into the registers, the drive answers with 512 bytes, and inside are
thirty numbered attributes whose numbering is standard only by convention.

`diskseer` issues `SMART_RCV_DRIVE_DATA` with `SENDCMDINPARAMS`, checks the
driver and IDE status bytes in the reply before trusting a single byte of it,
and parses the attribute table.

The payoff is precision Windows cannot offer. Windows says "uncorrected
errors". The drive distinguishes sectors *already replaced* (attr. 5) from
sectors *unreadable right now* (197) from sectors *given up on* (198) from
errors that happened *on the cable* (199) — four different situations needing
four different answers.

This path **does** require administrator rights: the drive accepts the command
only on a handle opened with `GENERIC_READ | GENERIC_WRITE`, and Windows grants
that on `\\.\PhysicalDriveN` only to administrators.

### SATA behind a USB bridge — SCSI-ATA translation

A drive in an external enclosure is not reachable with the ATA commands used
for internal drives: between the computer and the drive sits a bridge speaking
SCSI on one side and ATA on the other. The ATA command has to be wrapped inside
a SCSI one — an envelope inside an envelope — which the bridge unwraps and
hands to the drive.

Both the 12-byte and 16-byte command formats are attempted, since neither is
universal. Cheap enclosures often do not implement translation at all, and some
accept the command and return a block of zeroes rather than admitting it — so
the reply is validated before it is trusted.

### Counter availability, measured

| | SATA HDD | NVMe SSD | SATA SSD over USB |
|---|---|---|---|
| Access path | ATA IOCTL | NVMe log page | SCSI-ATA translation |
| Administrator required | yes | **no** | yes |
| Temperature | yes | yes | yes |
| Power-on hours | yes | yes | yes |
| Reallocated / pending sectors | yes | — | yes |
| Cable transfer errors | yes | — | yes |
| Unexpected power loss | — | yes | yes |
| Life used / spare capacity | — | yes | vendor-specific |
| Throttling minutes | — | yes | — |

## How it is built

Three layers, deliberately separated:

```
internal/collect   reads raw data from the machine — one file per OS
internal/rules     turns a snapshot into findings — where the knowledge lives
internal/report    renders findings for humans — terminal and HTML
```

The rules engine is a pure function: snapshot in, findings out. It touches
nothing. That is what makes it testable against saved captures of machines
nobody has to own, and what makes adding Linux a matter of writing one
collector while the diagnostic logic stays untouched.

### Design decisions worth stealing

**Missing data is not zero.** Every value the OS may fail to provide is a
pointer. A drive with zero errors is healthy; a drive whose error count could
not be read is unknown. Conflating the two is how diagnostic tools lie.
`internal/collect.Normalize` throws away zeroes that are provably meaningless —
wear on a mechanical drive, a peak temperature of 0 °C, anything the USB bridge
answered on the drive's behalf — and leaves the rest alone. Guessing further
would be worse than admitting the gap.

**No `-EncodedCommand`.** The PowerShell probe is written to a temp file and
run with `-File`. Base64-encoded PowerShell is a well-known malware signature
and trips antivirus and EDR. A diagnostic tool that sets off the customer's
antivirus never gets opened twice.

**Thresholds are the product, and they get corrected by real hardware.** Every
threshold lives at the top of its rules file. One example: any non-zero cable
error count started as a warning, until a perfectly healthy drive with 1833
powered hours turned up showing exactly one. A tool that cries wolf teaches
people to ignore real alarms, so anything under ten is now recorded, not
flagged.

**Vendor attributes stay unnamed unless they are unambiguous.** SSD attribute
IDs above 200 are defined by the controller vendor, not by a standard. ID 202
means "life remaining" on one brand and "address mark errors" on another. A
wrong label is worse than no label: an unnamed number gets looked up, a
mislabelled one gets believed.

**Same number, opposite advice.** Unexpected power loss on an internal drive
means check the power supply. On an external one it means the user unplugs it
without safe removal. The rule looks at how the drive is attached before it
speaks, because wrong advice costs more credibility than no advice.

### Test data

`testdata/` holds real captures run through `--anonymous`: manufacturer, model,
CPU and drive names replaced, timestamps zeroed, **every measurement left
untouched**. `tools/capture.ps1` anonymises by default — a capture taken on a
customer's machine describes their computer, not the fault being studied.

The whole suite runs against those anonymised fixtures and passes, which is the
practical proof that anonymising changes no measurement.

There is also a test that runs the entire rule catalogue in both languages and
fails if any finding comes out identical in both — the standard way a
bilingual program rots is a translation quietly left behind.

## How this was built

Written in Go over a few days, with **Claude Opus 5** as a pair programmer for
the whole thing — architecture, the Windows syscall work, and the rules
themselves. Several of the diagnoses in this tool exist because a bug or a
wrong threshold got caught in review rather than in production:

- the SATA attribute parser read the raw counter from byte 0 instead of byte 5,
  producing perfectly plausible numbers like "431174464261 reallocated
  sectors". It was caught by hand-decoding the bytes, and there is now a
  regression test that would have caught it immediately;
- teaching `isFlash` to recognise SSDs behind USB bridges silently reopened the
  "zero means unknown" hole for those drives — fixed, with its own test;
- the cable-error threshold was recalibrated after real hardware showed it
  would have fired on a healthy drive.

Low-level Windows work is unforgiving in a specific way: get an offset wrong
and you do not get an error, you get plausible wrong data. Every structure
offset in `internal/collect` is annotated for that reason.

## Status

Early but usable, and used in actual repair work. Windows only. Linux support
means writing one collector — `smartctl` and `/sys` — with the rules unchanged.

Issues and real-world captures of failing drives are especially welcome: the
thresholds only get better when they meet hardware that actually broke.

## License

MIT
