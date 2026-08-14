# Dock Invoice

[![CI](https://github.com/acuments/dock-invoice/actions/workflows/build.yml/badge.svg)](https://github.com/acuments/dock-invoice/actions/workflows/build.yml)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](LICENSE)
[![Sample export PDF](https://img.shields.io/badge/sample%20PDF-export%20invoice-blue)](https://github.com/acuments/dock-invoice/blob/main/fixtures/sample-export.pdf)

A self-contained desktop utility (Go + [Fyne](https://fyne.io)) that produces Indian GST tax
invoices, reproducing the layout of a reference export invoice (`INV-01.pdf`) faithfully while making every static field
(sender, bank, logo/signature, HSN/SAC defaults, numbering) a one-time configuration instead of
something retyped per invoice.

Supports three invoice treatments: export under LUT/bond, export with IGST paid, and domestic
(CGST+SGST or IGST, auto-selected — see "Domestic tax split" below). Amounts are entered in USD;
INR is derived from a per-invoice conversion factor and all tax is computed on the INR value.

### Sample output

Synthetic fixtures only — no real company or customer data:

| PDF | Layout | Treatment |
|-----|--------|-----------|
| [sample-export.pdf](fixtures/sample-export.pdf) | Modern | Export under LUT |
| [sample-domestic.pdf](fixtures/sample-domestic.pdf) | Classic | Domestic CGST+SGST |

UI screenshots: [fixtures/visual/](fixtures/visual/)

### Domestic tax split

A domestic supply is intra-state when the seller and the buyer are in the same state, in which case
the rate is split equally into CGST and SGST (18% becomes 9% + 9%); otherwise the whole rate is
charged as IGST. Either way the total tax is identical — only the split changes.

Each party's state is read from the **first two digits of its GSTIN**, which are the GST state code.
Each side falls back when its GSTIN is missing or malformed: the seller falls back to the State code
in Settings, and the buyer falls back to the place-of-supply code (which is the only signal available
for an unregistered buyer, who has no GSTIN at all). With nothing identifiable on either side the
invoice stays on IGST rather than asserting an intra-state supply nobody stated.

Where a registered buyer's GSTIN state and the place of supply disagree, the GSTIN wins.

### PDF layout: Modern or Classic

Every invoice PDF renders in one of two layouts, chosen per invoice:

| Layout | Look | Source |
|--------|------|--------|
| **Modern** (default) | Accent-coloured letterhead, the original design | `internal/pdf/bands.go`, `table.go`, `hsn.go` |
| **Classic** | Monochrome, fully-ruled "Tally-style" GST tax invoice — every field boxed in black rules, no colour | `internal/pdf/classic.go` |

Classic exists for domestic buyers who print on a black & white office printer: the Modern layout's
accent colours and unruled sections read poorly in mono, while Classic's continuous grid (reproduced
from a real Tally-style invoice) reproduces cleanly on any printer.

The default lives in **Settings → Defaults → Invoice layout**; every new invoice starts from it. Each
invoice's **Invoice details** section in the editor has its own **Layout** field to override that
default for one invoice — e.g. keeping Modern on an export invoice while the Settings default has
switched to Classic for domestic buyers.

Invoices saved before this feature existed have no layout stamped on them at all; they keep printing
in Modern (`model.NormalizeLayoutStyle` treats an empty `LayoutStyle` as Modern), so nothing already
issued changes look.

On Classic, the Declaration box always prints — the reference format carries one as a permanent part
of the form. If **Settings → Declarations** leaves the relevant invoice type's wording blank, Classic
falls back to standard wording ("We declare that this invoice shows the actual price of the goods
described and that all particulars are true and correct.") rather than printing an empty box.

## Project layout

```
internal/
  model/      domain types (Settings, Company, Bank, Customer, Item, Invoice, LineItem)
  money/      int64 minor-unit amounts, Indian/Western digit grouping, exact rounding
  calc/       per-line and invoice totals, LUT/IGST/CGST-SGST tax logic
  words/      number-to-words (Indian lakh/crore for INR, dollars/cents for USD)
  store/      SQLite persistence (modernc.org/sqlite, pure Go — no CGO needed for this part)
  numbering/  invoice number pattern expansion + financial-year rollover
  org/        the organization registry — which businesses exist, which one is open
  pdf/        the PDF renderer (github.com/go-pdf/fpdf) + embedded Noto Sans fonts
                classic.go the monochrome, fully-ruled Classic layout (see "PDF layout" above)
  ui/         the Fyne desktop UI: Invoices / Editor / Settings / Masters / About screens
                orgs.go    the organization manage dialog (add/rename/remove/open)
                workspace.go  startup: resolve the registry, open the active organization
                design.go  the design system — spacing scale, type scale, surfaces, badges
                theme.go   the Fyne theme: warm-paper light palette + warm-charcoal dark
main.go       GUI entry point, plus a --render-fixture dev flag (see below)
fixtures/     synthetic sample_invoice.json + rendered sample PDFs (safe to commit)
              visual/     UI screenshots for docs (see "Design" below)
testdata/     local-only — real invoices, org DBs, dev screenshots (gitignored)
```

## Design

The UI is styled as stationery rather than as a dashboard: a warm off-white
canvas, white "sheets" carrying the content, ink-black text, and one deep
ink-blue accent. Two rules keep it coherent, and both are enforced by
`internal/ui/design.go`:

1. **Hierarchy comes from type size and weight, never from colour.** The accent
   is spent only on things you can act on. (Fyne paints `widget.Label` with
   `HighImportance` in the primary colour, which is why screen titles used to
   render as small blue words that read like hyperlinks.)
2. **Vertical space is chosen, not inherited.** `widget.Label` carries its own
   inner padding, so stacking labels yields gaps nobody picked. Headings and
   captions are `canvas.Text` (no padding) laid out with the explicit spacing
   scale; where a wrapping `Label` is unavoidable, `tight()` cancels its inset.

Set `INVOICER_THEME=dark` for the warm-charcoal palette.

### Reviewing the UI visually

```sh
go test -run TestVisual_Screens ./internal/ui -v
```

renders every screen headlessly **under the real application theme** and writes
PNGs to `testdata/visual/` (gitignored). For committed doc screenshots use
`INVOICER_VISUAL_DIR=fixtures/visual`. Point output elsewhere with `INVOICER_VISUAL_DIR`, and
combine it with `INVOICER_THEME=dark` to check the dark palette:

```sh
INVOICER_THEME=dark INVOICER_VISUAL_DIR=/tmp/dark go test -run TestVisual_Screens ./internal/ui
```

## Requirements

- Go 1.26.5 (see `go.mod`)
- A C compiler (CGO is required to build/run the GUI — see "Packaging" below)

No other runtime dependencies: the database is pure-Go SQLite and the PDF renderer is pure-Go.

## Getting the source

```sh
git clone https://github.com/acuments/dock-invoice.git
cd dock-invoice
```

The Go module path is **`dock-invoice`** (same as the repo name). Clone into any directory you like —
imports always use `dock-invoice/...`, not the folder name on disk.

## Running

```sh
go run .
```

opens the desktop app. On first run it seeds sensible default settings (numbering patterns,
declaration text) — visit the **Settings** tab to fill in your company, bank, logo/signature, and
output folder; the **Masters** tab to add customers and saved items; then use **Invoices → New** to
create your first invoice.

**macOS Dock icon:** `go run .` launches a raw Unix executable, so macOS shows the generic **exec**
placeholder in the Dock. For the real app icon, package and run the `.app` bundle:

```sh
./build/package-macos.sh
open dist/Dock\ Invoice\ Generator.app
```

The packaged app is what you ship to customers; it embeds `Icon.png` into the bundle.

The database and config live at `os.UserConfigDir()/InvoiceGenerator/invoicer.db` (e.g.
`~/Library/Application Support/InvoiceGenerator/invoicer.db` on macOS). Override the path — e.g. for
a scratch/test run — with:

```sh
INVOICER_DB_PATH=/tmp/scratch.db go run .
```

### Multiple organizations

If you invoice for more than one business, each gets its own **organization**. **Organizations…**,
top right, lists them: **Open** switches to one, and the same dialog adds, renames, and removes them.
The window title names whichever organization is currently open, and the app reopens on it next
launch.

Organizations are fully separate — each has its own invoices, customers, saved items, invoice number
series, company details, bank account, and settings, in its own database file. Nothing is shared, so
one business's invoices can never appear in another's list or number series.

| | |
|---|---|
| Registry | `<data dir>/organizations.json` |
| First organization | `<data dir>/invoicer.db` |
| Organizations added later | `<data dir>/orgs/<name>.db` |

**Upgrading from a single-organization version:** nothing moves. Your existing `invoicer.db` becomes
your first organization on first launch, named after the company in your Settings, and stays exactly
where it is on disk.

**Removing** an organization takes it off the list but leaves its database file untouched — finalised
tax invoices have to be retained, so removal is never a way to delete them. The app tells you where
the file remains.

Setting `INVOICER_DB_PATH` pins the app to that one database and hides the Organizations button: an
explicit path means that file, not "whichever organization was last open".



### Dev-only: render a fixture straight to PDF, no UI

```sh
go run . --render-fixture fixtures/sample_invoice.json --out /tmp/out.pdf
```

This is the fastest way to iterate on the PDF layout without going through the UI or a database —
see `fixtures/sample_invoice.json` for the fixture schema (a human-readable form with decimal-string
amounts, converted internally via the same exact-decimal parsers the UI uses).

## Testing

```sh
go test ./...
```

covers the tax/money/words/numbering golden cases (matching the sample fixture's numbers
exactly), a SQLite round-trip proving invoices snapshot customer data (a later master edit never
changes a saved invoice), PDF rendering for all three invoice types plus a 20-line multi-page case
in both the Modern and Classic layouts, and headless UI tests (`fyne.io/fyne/v2/test`) covering
recalculation-on-keystroke, invoice-type field visibility, clone numbering, and validation.

## Packaging

**Fyne requires CGO.** That means you cannot produce a working GUI binary with a plain `go build` cross-compile from macOS to Windows/Linux — the platform GUI libraries and CGO toolchain must match the target OS.

### Prerequisites (all platforms)

1. Install Go 1.26.5 (see `go.mod`).
2. Install a C compiler:
   - **macOS:** Xcode Command Line Tools (`xcode-select --install`)
   - **Windows:** MinGW-w64 or MSVC (Visual Studio Build Tools)
   - **Linux:** `gcc` and development headers (`build-essential` on Debian/Ubuntu)
3. For cross-building Windows/Linux from macOS: **Docker** (for `fyne-cross`).

### One-command release build

`build/build.sh` automates the full pipeline: tests, native packaging, optional
cross-builds (when Docker is available), and versioned `.zip` / `.tar.gz` archives.

```sh
# Default: on macOS with Docker → darwin + windows + linux; otherwise native only
./build/build.sh

# Explicit targets
./build/build.sh all              # every platform this host can build
./build/build.sh native           # host OS only (fast iteration)
./build/build.sh darwin           # macOS .app (Mac only)
./build/build.sh windows linux    # via fyne-cross (Docker)

# Options
./build/build.sh native --skip-tests
./build/build.sh all --out /tmp/releases
./build/build.sh native --no-archive   # binaries only, no zip/tar.gz
./build/build.sh all --no-cross        # skip Docker cross-builds
```

Output lands under `dist/<app-slug>-<version>-b<build>/`:
- `staging/` — raw `.app`, `windows-*`, `linux-*` folders from fyne / fyne-cross
- `archives/` — distributable `*.zip` and `*.tar.gz` named with version and platform

Metadata (name, version, build, icon, bundle ID) is read from `FyneApp.toml`.

Legacy wrappers still work: `build/package-macos.sh`, `build/package-cross.sh`.

### macOS build (native)

Run on a Mac:

```sh
./build/package-macos.sh
```

This installs `fyne` if needed, runs `fyne package -os darwin -release`, and moves `Dock Invoice Generator.app` into `dist/`. Double-click the `.app` to run.

### Releases

Pre-built binaries are attached to [GitHub Releases](https://github.com/acuments/dock-invoice/releases) when a version tag (`v*`) is pushed:

| Asset | Platform |
|-------|----------|
| `dock-invoice-macos.zip` | macOS (`.app` bundle) |
| `dock-invoice-windows.zip` | Windows (`.exe`) |
| `dock-invoice-linux.tar.xz` | Linux |

The CI workflow (`.github/workflows/build.yml`) runs tests on push to `main`, pull requests, and manual dispatch. Binary packaging and GitHub Release publishing run only when you push a `v*` tag.

### Windows build

**Option A — GitHub Releases (recommended):** Download `dock-invoice-windows.zip` from the [latest release](https://github.com/acuments/dock-invoice/releases/latest), or push a `v*` tag to trigger a new build.

**Option B — GitHub Actions artifacts:** Push a `v*` tag. The `package` job on `windows-latest` uploads `dock-invoice-windows.zip`.

**Option C — fyne-cross on any host with Docker:**

```sh
./build/package-cross.sh windows
```

Output lands in `dist/` (copied from `fyne-cross/dist/windows-amd64/` etc.).

**Option D — Native on Windows:** Install Go and a C compiler, then:

```sh
go install fyne.io/tools/cmd/fyne@latest
fyne package -os windows -release
```

### Linux build

**Option A — GitHub Releases (recommended):** Download `dock-invoice-linux.tar.xz` from the [latest release](https://github.com/acuments/dock-invoice/releases/latest).

**Option B — GitHub Actions artifacts:** Push a `v*` tag; the `package` job on `ubuntu-latest` uploads `dock-invoice-linux.tar.xz`.

**Option C — fyne-cross:**

```sh
./build/package-cross.sh linux
```

**Option D — Native on Linux:**

```sh
go install fyne.io/tools/cmd/fyne@latest
fyne package -os linux -release
```

### Build all platforms at once

```sh
# macOS (native, on a Mac)
./build/package-macos.sh

# Windows + Linux (Docker required)
./build/package-cross.sh all
```

Both packaging scripts install their required CLI (`fyne`, `fyne-cross`) on demand if not already on `PATH`.

`Icon.png` and `FyneApp.toml` at the repo root define the app name, version, bundle ID, and icon used by `fyne package`. Update `FyneApp.toml` before each release (version, build number, support URL).

**Replacing the app icon:** overwrite `Icon.png` (1024×1024 PNG recommended), then:

```sh
./build/sync-icon.sh          # sync into embedded UI asset
./build/build.sh darwin       # repackage .app with new Dock icon
open dist/*/staging/*.app
```

`build/build.sh` runs the sync automatically; `sync-icon.sh` is for when you only changed the image.

## Backing up your data

All invoices, customers, saved items, settings, and invoice numbering for one organization live in a
**single SQLite file**:

| Platform | Default path |
|----------|--------------|
| macOS | `~/Library/Application Support/InvoiceGenerator/invoicer.db` |
| Windows | `%AppData%\InvoiceGenerator\invoicer.db` |
| Linux | `~/.config/InvoiceGenerator/invoicer.db` |

If you have [multiple organizations](#multiple-organizations), back up the whole
`InvoiceGenerator` folder — the additional organizations live in `orgs/` alongside
`organizations.json`, and **About → Back up data…** only copies the organization currently open.

Generated PDFs are stored separately in the output folder you choose under **Settings → Company** (default: `~/Documents/Invoices`).

### In the app

Open the **About** tab → **Your data** → **Back up data…** and save `invoicer-backup-YYYY-MM-DD.db` to a USB drive, cloud folder, or any location. To restore, use **Restore from backup…** (the app closes after a successful restore — reopen it to continue).

### Manually

Copy `invoicer.db` to a pendrive or backup location while the app is closed. To restore, replace the file and reopen the app. You can also point the app at a database on external storage for day-to-day use:

```sh
INVOICER_DB_PATH=/Volumes/MyUSB/invoicer.db ./Dock\ Invoice\ Generator.app/Contents/MacOS/dock-invoice
```

Keep logo and signature image files backed up separately if you reference them by path in Settings.

## Software updates and your data

Updates **do not wipe** your database. On startup the app opens the existing `invoicer.db` and applies any schema migrations automatically (for example, adding a new column to a table). Settings and invoices are stored as JSON blobs, so new app versions can add fields without rewriting old records — missing fields deserialize as empty/zero values.

Invoice snapshots are immutable: editing a customer or settings entry never changes invoices already saved.

**Recommended before a major upgrade:** use **About → Back up data…** so you have a restore point. If an update ever fails to open your database, restore from that backup file.

## License

Copyright © 2026 Ronak Gothi.

This project is licensed under the [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0).
If you modify this software and make it available to users over a network, you must provide
corresponding source code under the same license.

## Fonts

`internal/pdf/assets/NotoSans-{Regular,Bold}.ttf` are Noto Sans, licensed under the SIL Open Font
License (`internal/pdf/assets/OFL.txt`), registered via `fpdf`'s `AddUTF8FontFromBytes` so that ₹
(U+20B9) renders correctly on every platform without depending on a system font. The same embedded
font is reused for the Fyne UI's theme (`internal/ui/theme.go`) so ₹ displays identically in the
form and in the generated PDF.

## Known limitations / notes

- The generated PDF layout is a close visual match to the reference document, tuned against actual
  measured text widths rather than pixel-identical coordinates.
- `Icon.png` / `FyneApp.toml` — app branding and release metadata (see "Packaging" above).
- GSTR-1 CSV export and multiple sender profiles are explicitly out of scope (per the project plan).
