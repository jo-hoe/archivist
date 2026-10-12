package taxonomy

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanProviderWalksDirs(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "invoice", "car"))
	mustMkdir(t, filepath.Join(root, "invoice", "house"))
	mustMkdir(t, filepath.Join(root, "insurance"))

	p := NewScanProvider(root, map[string]string{
		"invoice":     "Bills and receipts",
		"invoice/car": "Vehicle invoices",
	})
	ctx := context.Background()

	r, err := p.Root(ctx)
	if err != nil {
		t.Fatalf("Root: %v", err)
	}
	top, err := p.Children(ctx, r)
	if err != nil {
		t.Fatalf("Children: %v", err)
	}
	if len(top) != 2 || top[0].Name != "insurance" || top[1].Name != "invoice" {
		t.Fatalf("expected sorted [insurance invoice], got %v", names(top))
	}
	if top[1].Description != "Bills and receipts" {
		t.Fatalf("description not attached: %q", top[1].Description)
	}

	sub, err := p.Children(ctx, top[1]) // invoice
	if err != nil {
		t.Fatalf("Children(invoice): %v", err)
	}
	if len(sub) != 2 || sub[0].Name != "car" || sub[0].Path != "invoice/car" {
		t.Fatalf("unexpected invoice children: %v", names(sub))
	}
	if sub[0].Description != "Vehicle invoices" {
		t.Fatalf("nested description not attached: %q", sub[0].Description)
	}
}

func TestScanProviderDescribeFallsBackToName(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "misc"))
	p := NewScanProvider(root, nil)
	ctx := context.Background()
	r, _ := p.Root(ctx)
	kids, _ := p.Children(ctx, r)
	if kids[0].DescribeOr() != "misc" {
		t.Fatalf("expected name fallback, got %q", kids[0].DescribeOr())
	}
}

func TestDeclaredProvider(t *testing.T) {
	yml := []byte(`
name: root
children:
  - name: invoice
    description: Bills
    children:
      - name: car
        description: Vehicle invoices
  - name: insurance
    description: Policies
`)
	p, err := NewDeclaredProvider(yml)
	if err != nil {
		t.Fatalf("NewDeclaredProvider: %v", err)
	}
	ctx := context.Background()
	r, _ := p.Root(ctx)
	top, _ := p.Children(ctx, r)
	if len(top) != 2 {
		t.Fatalf("expected 2 top children, got %d", len(top))
	}
	var invoice *Node
	for _, n := range top {
		if n.Name == "invoice" {
			invoice = n
		}
	}
	if invoice == nil {
		t.Fatal("invoice node missing")
	}
	sub, _ := p.Children(ctx, invoice)
	if len(sub) != 1 || sub[0].Name != "car" || sub[0].Path != "invoice/car" {
		t.Fatalf("unexpected declared child: %+v", sub)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func names(nodes []*Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}
