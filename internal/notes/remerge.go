package notes

import (
	"context"
	"encoding/json"

	"mint/internal/config"
)

// MergeInputs is what a re-merge of a chunked run's part notes is composed from: the
// release's instructions without any one-time context, the Change Map, and every
// part's notes in chunk order.
type MergeInputs struct {
	instructions, changeMap string
	partNotes               []string
}

type mergeInputsJSON struct {
	Instructions string   `json:"instructions"`
	ChangeMap    string   `json:"change_map"`
	PartNotes    []string `json:"part_notes"`
}

// MarshalJSON encodes in so a re-merge can be composed from it after a round trip.
func (in MergeInputs) MarshalJSON() ([]byte, error) {
	return json.Marshal(mergeInputsJSON{Instructions: in.instructions, ChangeMap: in.changeMap, PartNotes: in.partNotes})
}

// UnmarshalJSON decodes merge inputs MarshalJSON encoded.
func (in *MergeInputs) UnmarshalJSON(data []byte) error {
	var doc mergeInputsJSON
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	*in = MergeInputs{instructions: doc.Instructions, changeMap: doc.ChangeMap, partNotes: doc.PartNotes}
	return nil
}

// WithMergeInputs returns a copy of g that hands keep the merge inputs of each
// chunked run it writes; g itself is unchanged. A nil keep is handed nothing.
func (g *Generator) WithMergeInputs(keep func(MergeInputs)) *Generator {
	c := *g
	c.keepMergeInputs = keep
	return &c
}

// Remerge makes one merge call over in's part notes, with oneTimeContext appended to
// the instructions it carries, and returns its output byte-identical. A merge prompt
// over cfg.MaxPromptBytes is not sent and fails as ErrMergeTooLarge, as does a call
// whose failure is classified SIZE.
func (g *Generator) Remerge(ctx context.Context, in MergeInputs, cfg config.Config, oneTimeContext string) (string, error) {
	mergePrompt := composeMergePrompt(appendOneTimeContext(in.instructions, oneTimeContext), in.changeMap, in.partNotes)
	if len(mergePrompt) > cfg.MaxPromptBytes {
		return "", ErrMergeTooLarge
	}
	return g.sendMerge(ctx, mergePrompt, cfg.MaxPromptBytes)
}

// chunkedBody passes a chunked run's outcome on, handing keepMergeInputs the run's
// merge inputs once it is written; instructions are the release's, without the run's
// one-time context.
func (g *Generator) chunkedBody(instructions, changeMap, body string, partNotes []string, err error) (string, error) {
	if err != nil {
		return "", err
	}
	if g.keepMergeInputs != nil {
		g.keepMergeInputs(MergeInputs{instructions: instructions, changeMap: changeMap, partNotes: partNotes})
	}
	return body, nil
}
