Laravel plugin gains guidance for event-sourced models and long-running workflows.

✨ Added
- `laravel-event-sourcing` skill — aggregates that act as their own state machine, with projectors for read models and reactors for side effects, on spatie/laravel-event-sourcing.
- `laravel-workflows` skill — long-running processes on Durable Workflow that only sequence, with each step an activity calling an action.
- Copyable event-sourcing helpers — a guard trait, an `OperationAllowedFrom` attribute and an `IllegalTransition` exception for checking which states an operation is legal from.

🔧 Changed
- The actions, architecture, enums, jobs and state-machines skills now point to the event-sourcing and workflow skills, including that an aggregate guards instead of the action.
- The state-machines skill now applies only to models that aren't event-sourced.
