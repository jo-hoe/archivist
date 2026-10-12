package descent

import (
	"context"
	"testing"

	"github.com/jo-hoe/archivist/internal/model"
	"github.com/jo-hoe/archivist/internal/model/modeltest"
	"github.com/jo-hoe/archivist/internal/taxonomy"
)

// buildTree returns the root/invoice/{car,house}, insurance taxonomy used in the plan.
func exampleTree() *taxonomy.DeclaredProvider {
	p, _ := taxonomy.NewDeclaredProvider([]byte(`
name: root
children:
  - name: invoice
    description: Bills and receipts
    children:
      - name: car
        description: Vehicle invoices
      - name: house
        description: Home invoices
  - name: insurance
    description: Insurance policies
`))
	return p
}

func TestThresholdStopDescendsToLeaf(t *testing.T) {
	// Model confidently picks invoice, then car.
	fake := &modeltest.Fake{
		ChoiceFn: func(_ model.State, q model.ChoiceQuestion) (model.ChoiceAnswer, error) {
			if _, ok := q.Criteria["invoice"]; ok {
				return model.ChoiceAnswer{Choice: "invoice", Confidence: 0.95}, nil
			}
			return model.ChoiceAnswer{Choice: "car", Confidence: 0.9}, nil
		},
	}
	w := NewWalker(exampleTree(), ThresholdStop{Threshold: 0.6}, "")
	res, err := w.Descend(context.Background(), fake, model.State{Body: "tire change invoice"})
	if err != nil {
		t.Fatalf("Descend: %v", err)
	}
	if res.Node.Path != "invoice/car" || res.Depth != 2 || res.StoppedEarly {
		t.Fatalf("expected invoice/car depth 2, got path=%q depth=%d early=%v", res.Node.Path, res.Depth, res.StoppedEarly)
	}
}

func TestThresholdStopStopsOnLowConfidence(t *testing.T) {
	// Picks invoice confidently, but is unsure between car/house → stays at invoice.
	fake := &modeltest.Fake{
		ChoiceFn: func(_ model.State, q model.ChoiceQuestion) (model.ChoiceAnswer, error) {
			if _, ok := q.Criteria["invoice"]; ok {
				return model.ChoiceAnswer{Choice: "invoice", Confidence: 0.95}, nil
			}
			return model.ChoiceAnswer{Choice: "car", Confidence: 0.3}, nil
		},
	}
	w := NewWalker(exampleTree(), ThresholdStop{Threshold: 0.6}, "")
	res, err := w.Descend(context.Background(), fake, model.State{Body: "generic invoice"})
	if err != nil {
		t.Fatalf("Descend: %v", err)
	}
	if res.Node.Path != "invoice" || !res.StoppedEarly {
		t.Fatalf("expected to stop at invoice, got path=%q early=%v", res.Node.Path, res.StoppedEarly)
	}
}

func TestThresholdStopStaysAtRoot(t *testing.T) {
	fake := &modeltest.Fake{ChoiceFn: modeltest.ChoosePick("invoice", 0.2)}
	w := NewWalker(exampleTree(), ThresholdStop{Threshold: 0.6}, "")
	res, err := w.Descend(context.Background(), fake, model.State{Body: "???"})
	if err != nil {
		t.Fatalf("Descend: %v", err)
	}
	if res.Node.Path != "" || res.Depth != 0 || !res.StoppedEarly {
		t.Fatalf("expected to stay at root, got %+v", res)
	}
}

func TestStayHereStopInjectsOptionAndStops(t *testing.T) {
	// The model picks the synthetic stay-here option at the invoice level.
	fake := &modeltest.Fake{
		ChoiceFn: func(_ model.State, q model.ChoiceQuestion) (model.ChoiceAnswer, error) {
			if _, ok := q.Criteria["invoice"]; ok {
				// First level: ensure the synthetic option is present too.
				if _, has := q.Criteria[stayHereKey]; !has {
					t.Error("stay-here option should be injected")
				}
				return model.ChoiceAnswer{Choice: "invoice", Confidence: 0.95}, nil
			}
			return model.ChoiceAnswer{Choice: stayHereKey, Confidence: 0.95}, nil
		},
	}
	w := NewWalker(exampleTree(), StayHereStop{Threshold: 0.6}, "")
	res, err := w.Descend(context.Background(), fake, model.State{Body: "misc invoice"})
	if err != nil {
		t.Fatalf("Descend: %v", err)
	}
	if res.Node.Path != "invoice" || !res.StoppedEarly {
		t.Fatalf("expected stop at invoice via stay-here, got path=%q early=%v", res.Node.Path, res.StoppedEarly)
	}
}

func TestDescendStopsAtEmptyLeaf(t *testing.T) {
	// insurance has no children → descent ends there, not flagged as early stop.
	fake := &modeltest.Fake{ChoiceFn: modeltest.ChoosePick("insurance", 0.99)}
	w := NewWalker(exampleTree(), ThresholdStop{Threshold: 0.6}, "")
	res, err := w.Descend(context.Background(), fake, model.State{Body: "policy"})
	if err != nil {
		t.Fatalf("Descend: %v", err)
	}
	if res.Node.Path != "insurance" || res.StoppedEarly {
		t.Fatalf("expected natural stop at insurance leaf, got path=%q early=%v", res.Node.Path, res.StoppedEarly)
	}
}
