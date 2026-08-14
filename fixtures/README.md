# Sample fixtures (synthetic data only)

Everything committed here uses **fake** company, bank, GSTIN, and customer details
suitable for screenshots, sample PDFs, and `--render-fixture` development.

| File | Purpose |
|------|---------|
| `sample_invoice.json` | Export (LUT), Modern layout |
| `sample_invoice_domestic.json` | Domestic, Classic layout |
| `sample-export.pdf` | Rendered from `sample_invoice.json` |
| `sample-domestic.pdf` | Rendered from `sample_invoice_domestic.json` |
| `visual/*.png` | UI screenshots from `go test -run TestVisual ./internal/ui` |

Regenerate PDFs:

```sh
go run . --render-fixture fixtures/sample_invoice.json --out fixtures/sample-export.pdf
go run . --render-fixture fixtures/sample_invoice_domestic.json --out fixtures/sample-domestic.pdf
```

Regenerate UI screenshots (writes to `testdata/visual/` by default; copy to `fixtures/visual/` for release):

```sh
go test -run TestVisual ./internal/ui -v
INVOICER_VISUAL_DIR=fixtures/visual go test -run TestVisual ./internal/ui -v
```
