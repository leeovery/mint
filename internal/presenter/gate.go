package presenter

// Choice is one of the semantic answers an interactive gate can return. The four
// values below are the ENGINE's vocabulary — the engine owns what each choice
// means (accept, abort, edit, regenerate) and owns the e/r re-entry loop; the
// presenter only renders the declared set and returns one of these. Choice is a
// distinct string type (not a bare string) so a gate answer cannot be confused
// with arbitrary text at a call boundary.
type Choice string

// The four semantic choices. Not every gate declares all four — a gate carries
// only the choices it offers (see NotesReviewGate vs ReuseConfirmGate). These
// constants name the vocabulary; the declared SET lives on each Gate value, so no
// renderer hardcodes "y/n/e/r" — membership and order are always read from the
// gate via Has/Keys.
const (
	// ChoiceYes accepts the gated content and proceeds — the default-yes 99% path.
	ChoiceYes Choice = "y"
	// ChoiceNo aborts the run (the engine then auto-unwinds).
	ChoiceNo Choice = "n"
	// ChoiceEdit opens the notes in $EDITOR (engine-driven; presenter never invokes
	// $EDITOR itself).
	ChoiceEdit Choice = "e"
	// ChoiceRegen regenerates the notes with context (engine-driven; presenter never
	// invokes claude itself).
	ChoiceRegen Choice = "r"
)

// gateFailLabel is the FIXED label both presenters render for the
// forbidden-combination gate failure (non-TTY stdin without -y): "gate". It is the
// gate MECHANISM that is failing, not the gate's Subject (the notes content), so
// the label is this fixed word rather than gate.Subject — a reader sees the gate
// itself failed, not that "notes" failed.
const gateFailLabel = "gate"

// gateNotTTYMessageASCII is the byte-pure ASCII message text for the plain
// forbidden-combination failure: a semicolon form (NOT the em-dash) so the plain
// byte-purity guard stays green. The pretty presenter renders the spec's em-dash
// form (gateNotTTYMessagePretty) instead.
const gateNotTTYMessageASCII = "not a TTY; pass -y to run unattended"

// gateNotTTYMessagePretty is the spec's message text for the pretty
// forbidden-combination failure, with the em dash allowed in pretty mode (no
// byte-purity constraint there). The plain presenter uses the ASCII form above.
const gateNotTTYMessagePretty = "not a TTY — pass -y to run unattended"

// GateChoice pairs a Choice key with its human-facing action label (e.g.
// "accept & proceed"). The slice ORDER on a Gate is significant: it is the render
// order of the vertical menu (the spec lists y, n, e, r top-to-bottom), so a
// GateChoice carries no separate ordering field — position in Gate.Choices is the
// order.
type GateChoice struct {
	// Key is the semantic answer this menu line returns when chosen.
	Key Choice
	// Action is the engine/spec-supplied label rendered beside the key (verbatim).
	Action string
}

// Gate is a pure DATA model of one interactive prompt: the question text, the
// ordered set of choices it offers, and which choice is the default. It carries
// NO rendering — no fmt, no lipgloss — so both presenters render the same gate
// value their own way, and engine-driven tests can script answers against the
// model alone.
//
// A gate is described by the choices it offers: Prompt(gate) renders WHATEVER set
// the gate declares and returns one of them, so a single method renders every
// gate variant (four-choice notes-review, two-choice reuse confirm). Membership
// and order are read from the value via Has/Keys — nothing hardcodes the y/n/e/r
// set.
//
// The Question field (the "Continue?" prompt text) and the Presenter.Prompt
// METHOD share the word "prompt" in the spec but are deliberately kept in
// separate namespaces here: the field is named Question to avoid any confusion
// with the interface method Prompt(Gate).
//
// Default invariant: Default MUST be one of Choices' keys. The model does NOT
// assume a yes-default — a gate may declare any declared choice as its default
// (see the non-y-default case). The constructors below all declare default-yes
// per the spec, but the type itself is default-agnostic.
type Gate struct {
	// Question is the prompt text rendered last, below the menu (e.g. "Continue?").
	Question string
	// Subject names what this gate is accepting — "notes" for the notes-review and
	// reuse-confirm gates, "source"/"target" for regenerate's selection prompts. It
	// is the LEFT side of the -y auto-accept echo: when the gate is skipped under -y
	// the presenter renders "{Subject}: {AcceptEcho} (-y)" from this field, so the
	// echo subject is carried in the payload and NO renderer hardcodes "notes". It
	// plays no part in the interactive render (the menu reads Question and Choices),
	// only in the skip echo.
	Subject string
	// AcceptEcho is the word/value shown after the subject in the -y auto-accept
	// echo: the presenter renders "{Subject}: {AcceptEcho} (-y)" (plain) /
	// "✓ {Subject}  {AcceptEcho} (-y)" (pretty). For the notes/reuse gates it is the
	// fixed word "accepted" (so the echo reads "notes: accepted (-y)"); for the
	// source/target gates it is the CHOSEN value — the engine-set flag/default —
	// so a captured log shows which source/target was used ("source: github (-y)").
	// Carrying it in the payload generalises the echo word per gate; no renderer
	// hardcodes "accepted". The value must stay byte-pure ASCII for the plain echo
	// (the notes word and the option keys both are).
	AcceptEcho string
	// Choices is the ordered set of offered choices; the order is the render order.
	Choices []GateChoice
	// Default is the choice that fires on a deliberate empty Enter. It must be a
	// member of Choices.
	Default Choice
}

// Has reports whether c is a member of the gate's DECLARED choice set. It reads
// the gate's own choices — there is no hardcoded y/n/e/r list — so a choice the
// gate does not offer (e.g. ChoiceEdit on the two-choice reuse confirm) returns
// false.
func (g Gate) Has(c Choice) bool {
	for _, choice := range g.Choices {
		if choice.Key == c {
			return true
		}
	}
	return false
}

// Keys returns the gate's choice keys in declared (render) order. It is the
// ordered, key-only view of Choices used to assert and render the menu without
// reaching for a hardcoded set.
func (g Gate) Keys() []Choice {
	keys := make([]Choice, len(g.Choices))
	for i, choice := range g.Choices {
		keys[i] = choice.Key
	}
	return keys
}

// NotesReviewGate is the four-choice notes-review gate used on release and
// regenerate-fresh: y/n/e/r in that order, default-yes, with the spec's action
// labels. The engine owns the e/r re-entry loop — Prompt only renders this set
// and returns one key.
func NotesReviewGate() Gate {
	return Gate{
		Question: "Continue?",
		Subject:  "notes",
		// "accepted" keeps the notes echo "notes: accepted (-y)" after the echo word
		// was generalised onto AcceptEcho (the source/target gates carry their chosen
		// value here instead).
		AcceptEcho: "accepted",
		Choices: []GateChoice{
			{Key: ChoiceYes, Action: "accept & proceed"},
			{Key: ChoiceNo, Action: "abort"},
			{Key: ChoiceEdit, Action: "edit in $EDITOR"},
			{Key: ChoiceRegen, Action: "regenerate"},
		},
		Default: ChoiceYes,
	}
}

// ReuseConfirmGate is the reduced two-choice confirm used on regenerate that
// reuses existing notes: y/n only, default-yes, rendered in the same "Continue?"
// vocabulary. It declares NO e/r — there are no freshly-generated notes to edit
// or regenerate.
func ReuseConfirmGate() Gate {
	return Gate{
		Question: "Continue?",
		// The reuse confirm is also a notes-acceptance gate in the same Continue?
		// vocabulary, so its -y echo is "notes: accepted (-y)" — same subject and
		// AcceptEcho word as the notes-review gate.
		Subject:    "notes",
		AcceptEcho: "accepted",
		Choices: []GateChoice{
			{Key: ChoiceYes, Action: "accept & proceed"},
			{Key: ChoiceNo, Action: "abort"},
		},
		Default: ChoiceYes,
	}
}

// SourceGate is regenerate's interactive SOURCE selection prompt, expressed as a
// Gate so it flows through the same Prompt(gate) method as the notes gate — the
// shared line-read loop, the -y skip+echo, and the forbidden-combination fail-loud
// path. It is an ENUMERATED choice set: options are the engine-supplied available
// sources as GateChoice keys (no free-form value entry), and def is the
// engine-supplied default (from the source flag/default). The Question is the terse
// "Source?" form, which renders well as a plain key:value-style prompt line
// ("Source? [github/gitlab]"). AcceptEcho is the chosen value string(def), so the
// -y echo reads "source: {chosen} (-y)" and a captured log shows which source was
// used. The presenter invents none of these — they all arrive via the args.
//
// Precondition: the supplied options keys and def MUST be ASCII enumerated values
// only. The shared parseChoice case-folds the input (strings.ToLower) before
// matching it against these keys, and AcceptEcho = string(def) must stay byte-pure
// ASCII for the plain "{Subject}: {AcceptEcho} (-y)" echo. A non-ASCII source value
// would render the plain echo wrong (or trip the plain byte-purity guard) and would
// not case-fold reliably. This contract is NOT enforced here — it holds because the
// engine's source enumeration is ASCII; a future change that broadens source values
// beyond ASCII must revisit it.
func SourceGate(options []GateChoice, def Choice) Gate {
	return Gate{
		Question:   "Source?",
		Subject:    "source",
		AcceptEcho: string(def),
		Choices:    options,
		Default:    def,
	}
}

// TargetGate is regenerate's interactive TARGET selection prompt, the target
// counterpart of SourceGate: same enumerated-choice, shared-Prompt wiring. options
// are the engine-supplied available targets and def the engine-supplied default
// (from the target flag/default). The Question is the terse "Target?" form
// ("Target? [stable/beta]" in plain); Subject is "target"; AcceptEcho is the chosen
// value so the -y echo reads "target: {chosen} (-y)".
//
// Precondition: the supplied options keys and def MUST be ASCII enumerated values
// only, for the same reason as SourceGate. The shared parseChoice case-folds the
// input (strings.ToLower) before matching it against these keys, and
// AcceptEcho = string(def) must stay byte-pure ASCII for the plain
// "{Subject}: {AcceptEcho} (-y)" echo. A non-ASCII target value would render the
// plain echo wrong (or trip the plain byte-purity guard) and would not case-fold
// reliably. This contract is NOT enforced here — it holds because the engine's
// target enumeration is ASCII; a future change that broadens target values beyond
// ASCII must revisit it.
func TargetGate(options []GateChoice, def Choice) Gate {
	return Gate{
		Question:   "Target?",
		Subject:    "target",
		AcceptEcho: string(def),
		Choices:    options,
		Default:    def,
	}
}
