Adds a Go plugin and rewrites the Laravel skills so they state their rules with reasons and keep situational techniques separate.

✨ Added
- New Go plugin with eight skills covering architecture, actions, types, interfaces, errors, concurrency, testing and quality, installable with `npx agntc@latest add leeovery/agentic-skills/go`.
- Laravel multi-tenancy skill now covers stancl/tenancy v4, with a tenancy-aware test harness and reference files for the test setup.
- Laravel packages skill now covers building a package with package-tools, a manager with drivers, a shipped fake and Testbench tests.
- Laravel quality skill ships a house `pint.json` and a `rector.php` to copy into projects.
- Laravel routing skill ships a `ConditionalRouteBinder` for routes that need different model lookups.
- Laravel services skill adds a full Saloon driver walkthrough with config, connector, requests and tests.
- Laravel testing skill adds model factory and test setup references.
- Laravel value objects skill adds a money reference built on moneyphp/money.

🔧 Changed
- Laravel skills are rewritten around explicit rules with their reasons, and situational techniques now sit in a "reach for these when the situation calls for it" section.
- Laravel architecture skill is now a map of which class does which job, with a flat `app/` layout for new projects.
- Laravel controllers, routing and DTO skills now use singular controllers, explicit named routes, `->can()` on every route, and explicit typed DTO mapping through per-surface transformers.
- Laravel validation, testing and quality skills now use Form Requests that only validate, `test()` with `will`/`can` names, container mocks, and Pint, Rector and type coverage instead of PHPStan by default.
- Laravel exceptions skill now has domain exceptions carry their own status code and reporting rule.
- Laravel README updated for the new skill descriptions, and the repository README now lists the Go plugin alongside Laravel and Nuxt.

🗑️ Removed
- Laravel reference files that duplicated skill content, including the architecture references, `laravel-quality` code-style and type-safety references, and the old validation, jobs and routing guides.
