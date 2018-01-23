package trace

import (
	"testing"

	depgraph "github.com/openshift/origin/tools/depcheck/pkg/graph"
)

var pkgs = &PackageList{
	Packages: []Package{
		{
			Dir:        "/path/to/github.com/test/repo/root",
			ImportPath: "github.com/test/repo/root",
			Imports: []string{
				"github.com/test/repo/pkg/one",
			},
		},
		{
			Dir:        "/path/to/github.com/test/repo/pkg/one",
			ImportPath: "github.com/test/repo/pkg/one",
			Imports: []string{
				"github.com/test/repo/pkg/two",
				"github.com/test/repo/pkg/three",
				"github.com/test/repo/pkg/depends_on_fmt",
			},
		},
		{
			Dir:        "/path/to/github.com/test/repo/pkg/two",
			ImportPath: "github.com/test/repo/pkg/two",
			Imports: []string{
				"github.com/test/repo/vendor/github.com/testvendor/vendor_one",
			},
		},
		{
			Dir:        "/path/to/github.com/test/repo/pkg/three",
			ImportPath: "github.com/test/repo/pkg/three",
			Imports: []string{
				"github.com/test/repo/shared/shared_one",
			},
		},
		{
			Dir:        "/path/to/github.com/test/repo/pkg/depends_on_fmt",
			ImportPath: "github.com/test/repo/pkg/depends_on_fmt",
			Imports: []string{
				"fmt",
				"github.com/test/repo/unique/unique_nonvendored_one",
			},
		},
		{
			Dir:        "/path/to/github.com/test/repo/unique/unique_nonvendored_one",
			ImportPath: "github.com/test/repo/unique/unique_nonvendored_one",
			Imports:    []string{},
		},
		{
			Dir:        "/path/to/github.com/test/repo/shared/shared_one",
			ImportPath: "github.com/test/repo/shared/shared_one",
			Imports:    []string{},
		},
		{
			Dir:        "/path/to/github.com/test/repo/vendor/github.com/testvendor/vendor_one",
			ImportPath: "github.com/test/repo/vendor/github.com/testvendor/vendor_one",
			Imports: []string{
				"github.com/test/repo/unique/unique_vendor_one",
				"github.com/test/repo/shared/shared_one",
			},
		},
		{
			Dir:        "/path/to/github.com/test/repo/unique/unique_vendor_one",
			ImportPath: "github.com/test/repo/unique/unique_vendor_one",
			Imports:    []string{},
		},
	},
}

func TestBuildGraphCreatesExpectedNodesAndEdges(t *testing.T) {
	invalidImports := map[string]bool{
		"fmt": true,
	}

	g, err := BuildGraph(pkgs, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Nodes()) != len(pkgs.Packages) {
		t.Fatalf("node count mismatch. Expected %v nodes but got %v.", len(pkgs.Packages), len(g.Nodes()))
	}

	for _, pkg := range pkgs.Packages {
		from, exists := g.NodeByName(pkg.ImportPath)
		if !exists || !g.Has(from) {
			t.Fatalf("expected node with name to exist for given package with ImportPath %q", pkg.ImportPath)
		}

		for _, dep := range pkg.Imports {
			if _, skip := invalidImports[dep]; skip {
				continue
			}

			to, exists := g.NodeByName(dep)
			if !exists || !g.Has(to) {
				t.Fatalf("expected node with name ")
			}

			if !g.HasEdgeFromTo(from, to) {
				t.Fatalf("expected edge to exist between nodes %v and %v", from, to)
			}
		}
	}
}

func TestBuildGraphExcludesNodes(t *testing.T) {
	excludes := []string{
		"github.com/test/repo/pkg/three",
		"github.com/test/repo/pkg/depends_on_fmt",
	}

	g, err := BuildGraph(pkgs, excludes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, n := range g.Nodes() {
		node, ok := n.(*depgraph.Node)
		if !ok {
			t.Fatalf("expected node to be of type *depgraph.Node")
		}

		for _, exclude := range excludes {
			if node.UniqueName == exclude {
				t.Fatalf("expected node with unique name %q to have been excluded from the graph", node.UniqueName)
			}
		}
	}

}

func TestPackagesWithInvalidPathsAreOmitted(t *testing.T) {
	pkgList := &PackageList{
		Packages: []Package{
			{
				Dir:        "/path/to/github.com/test/repo/invalid",
				ImportPath: "invalid/import/path1",
				Imports: []string{
					"fmt",
					"invalid.import.path2",
					"invalid.import.path3",
				},
			},
			{
				Dir:        "/path/to/github.com/test/repo/invalid",
				ImportPath: "invalid.import.path2",
				Imports: []string{
					"net",
					"encoding/json",
				},
			},
			{
				Dir:        "/path/to/github.com/test/repo/invalid",
				ImportPath: "invalid3",
			},
		},
	}

	g, err := BuildGraph(pkgList, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Nodes()) != 0 {
		t.Fatalf("expected no nodes to have been created for an invalid package list. Saw %v unexpected nodes.", len(g.Nodes()))
	}
}

func TestLabelNamesForVendoredNodes(t *testing.T) {
	pkgList := &PackageList{
		Packages: []Package{
			{
				Dir:        "/path/to/github.com/test/repo/vendor/github.com/testvendor/vendor_one",
				ImportPath: "github.com/test/repo/vendor/github.com/testvendor/vendor_one",
			},
		},
	}

	expectedLabelName := "github.com/testvendor/vendor_one"

	g, err := BuildGraph(pkgList, []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(g.Nodes()) != 1 {
		t.Fatalf("expected graph of size 1, but got graph with %v nodes", len(g.Nodes()))
	}

	node, ok := g.Nodes()[0].(*depgraph.Node)
	if !ok {
		t.Fatalf("expected node %v to be of type *depgraph.Node", node)
	}

	if node.LabelName != expectedLabelName {
		t.Fatalf("expected node label name to be %q but was %q", expectedLabelName, node.LabelName)
	}
}
