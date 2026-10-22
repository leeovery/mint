✨ Added
- New `laravel-event-sourcing` skill — aggregates that act as their own state machine, with projectors and reactors, on spatie/laravel-event-sourcing.
- Copy-in reference files for event sourcing: a guard trait, an `#[OperationAllowedFrom]` attribute and an `IllegalTransition` exception.
- New `laravel-workflows` skill — long-running processes on Durable Workflow that only sequence, with each step an activity calling an action.

🔧 Changed
- The architecture map now lists aggregates, projectors, reactors, workflows and activities, and when to choose a workflow over a job chain.
- Actions, enums, jobs and state-machine skills now cross-link to the new skills and note where event-sourced models differ — the aggregate guards instead of the action, and stored events carry plain values.
- The state-machines skill is now scoped to models that aren't event-sourced.
- Plugin skill counts in the READMEs go from 20 to 22 for Laravel, with a new "Event Sourcing and Workflows" section.
