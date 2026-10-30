package presenter_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/muesli/termenv"

	"mint/internal/presenter"
	"mint/internal/presenter/presentertest"
)

func previewedGate() presenter.Gate {
	return presenter.Gate{
		Question:   "Exclude docs/design/ from the notes? (design documents; 2 files, 15 KB)",
		Subject:    "docs/design/",
		AcceptEcho: "kept",
		Choices: []presenter.GateChoice{
			{Key: presenter.ChoiceExclude, Action: "exclude"},
			{Key: presenter.ChoiceKeep, Action: "keep"},
		},
		Default: presenter.ChoiceKeep,
	}
}

func TestPlainPresenterPreviewGate_PrintsTheQuestionWithoutItsChoicesAndReadsNoInput(t *testing.T) {
	t.Parallel()

	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	in := strings.NewReader("e\n")
	p := presenter.NewPlainPresenterWithInput(out, errBuf, in)
	p.PreviewGate(previewedGate())

	want := "Exclude docs/design/ from the notes? (design documents; 2 files, 15 KB) (dry run: not asked)\n"
	if got := out.String(); got != want {
		t.Errorf("PreviewGate out\n got: %q\nwant: %q", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want nothing", errBuf.String())
	}
	if in.Len() != len("e\n") {
		t.Errorf("PreviewGate read %d bytes of input, want none", len("e\n")-in.Len())
	}
	assertBytePureASCIIStreams(t, "plain gate preview", out, errBuf)
}

func TestPrettyPresenterPreviewGate_ShowsTheQuestionAsItsGateWouldWithoutTheChoicesAndReadsNoInput(t *testing.T) {
	t.Parallel()

	out, errBuf := &bytes.Buffer{}, &bytes.Buffer{}
	in := strings.NewReader("e\n")
	p := presenter.NewPrettyPresenter(out, presenter.WithErr(errBuf), presenter.WithProfile(termenv.Ascii), presenter.WithInput(in))
	p.PreviewGate(previewedGate())

	want := "\nExclude docs/design/ from the notes? (design documents; 2 files, 15 KB)\ndry run: not asked\n"
	if got := out.String(); got != want {
		t.Errorf("PreviewGate out\n got: %q\nwant: %q", got, want)
	}
	if errBuf.Len() != 0 {
		t.Errorf("stderr = %q, want nothing", errBuf.String())
	}
	if in.Len() != len("e\n") {
		t.Errorf("PreviewGate read %d bytes of input, want none", len("e\n")-in.Len())
	}
}

func TestPreviewGate_LeavesTheSuccessLineStanding(t *testing.T) {
	t.Parallel()

	out, _ := drive(func(p *presenter.PlainPresenter) {
		p.PreviewGate(previewedGate())
		p.RunFinished(presenter.RunResult{Outcome: presenter.OutcomeReleased, Project: "acme", Version: "1.4.0"})
	})

	if want := "done: acme v1.4.0\n"; !bytes.HasSuffix(out.Bytes(), []byte(want)) {
		t.Errorf("plain output = %q, want it to end %q", out.String(), want)
	}
}

func TestRecordingPresenter_RecordsAPreviewedGateWithItsPayload(t *testing.T) {
	t.Parallel()

	rec := &presentertest.RecordingPresenter{}
	rec.PreviewGate(previewedGate())

	if len(rec.Events) != 1 || rec.Events[0].Kind != presentertest.KindPreviewGate {
		t.Fatalf("events = %v, want one PreviewGate", rec.Kinds())
	}
	if got := rec.Events[0].PreviewGate; got.Question != previewedGate().Question || !got.Has(presenter.ChoiceKeep) {
		t.Errorf("payload = %+v, want %+v", got, previewedGate())
	}
	if s := presentertest.KindPreviewGate.String(); s != "PreviewGate" {
		t.Errorf("kind string = %q, want PreviewGate", s)
	}
}
