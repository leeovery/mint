✨ Added
- New `laravel-event-sourcing` skill — aggregates that act as their own state machine, with projectors and reactors, on spatie/laravel-event-sourcing.
- Copy-in reference files for event sourcing: a state-guard trait, an `OperationAllowedFrom` attribute and an `IllegalTransition` exception.
- New `laravel-workflows` skill — long-running processes on Durable Workflow that only sequence, with each step an activity calling an action.

🔧 Changed
- Architecture map now routes event-sourced models and signal- or timer-driven processes to the new skills.
- The actions guard-first rule now makes an exception for event-sourced models, where the aggregate does the guarding.
- Enums, jobs and state-machines skills now cross-reference event sourcing and workflows, and the state-machines skill is scoped to models that aren't event-sourced.
- Plugin READMEs now list 22 Laravel skills, including a new "Event Sourcing and Workflows" group.
