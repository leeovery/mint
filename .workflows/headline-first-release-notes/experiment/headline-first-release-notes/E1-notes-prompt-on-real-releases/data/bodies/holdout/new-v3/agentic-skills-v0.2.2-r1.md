Laravel plugin now guides event-sourced models and long-running workflows.

✨ Added
- `laravel-event-sourcing` skill — aggregates that act as their own state machine, with projectors and reactors on spatie/laravel-event-sourcing.
- `laravel-workflows` skill — long-running processes on Durable Workflow that only sequence, with each step an activity calling an action.

🔧 Changed
- The actions, architecture, enums, jobs and state-machines skills now point to event sourcing and workflows, including where the guard lives on an event-sourced model.
- The `laravel-state-machines` skill now applies only to models that aren't event-sourced.
