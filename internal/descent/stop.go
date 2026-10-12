package descent

import (
	"github.com/jo-hoe/archivist/internal/model"
	"github.com/jo-hoe/archivist/internal/taxonomy"
)

// stayHereKey is the synthetic option StayHereStop injects to let the model
// explicitly decide to keep the file at the current level.
const stayHereKey = "__stay__"

// ThresholdStop descends while the model's top-choice confidence meets or
// exceeds Threshold; below it, the walk stops at the current folder. This is the
// default strategy.
type ThresholdStop struct {
	Threshold float64
}

// Prepare asks a plain Choice over the children, with no synthetic option.
func (ThresholdStop) Prepare(_ *taxonomy.Node, children []*taxonomy.Node) model.ChoiceQuestion {
	return buildChoice(children)
}

// Decide stops when confidence is below the threshold; otherwise descends into
// the chosen child.
func (s ThresholdStop) Decide(children []*taxonomy.Node, ans model.ChoiceAnswer) (*taxonomy.Node, bool) {
	if ans.Confidence < s.Threshold {
		return nil, true
	}
	next := childByName(children, ans.Choice)
	if next == nil {
		return nil, true
	}
	return next, false
}

// StayHereStop injects a synthetic "stay here" option into the Choice. The walk
// stops when the model picks it, or when confidence falls below Threshold.
type StayHereStop struct {
	Threshold float64
	// StayDescription is the criteria text for the synthetic option.
	StayDescription string
}

// Prepare adds the synthetic stay-here option to the children choice.
func (s StayHereStop) Prepare(current *taxonomy.Node, children []*taxonomy.Node) model.ChoiceQuestion {
	q := buildChoice(children)
	desc := s.StayDescription
	if desc == "" {
		desc = "None of the subfolders fit; keep the document in the current folder."
	}
	q.Criteria[stayHereKey] = desc
	q.Options = append(q.Options, stayHereKey)
	return q
}

// Decide stops on the synthetic option or on low confidence; otherwise descends.
func (s StayHereStop) Decide(children []*taxonomy.Node, ans model.ChoiceAnswer) (*taxonomy.Node, bool) {
	if ans.Choice == stayHereKey || ans.Confidence < s.Threshold {
		return nil, true
	}
	next := childByName(children, ans.Choice)
	if next == nil {
		return nil, true
	}
	return next, false
}
