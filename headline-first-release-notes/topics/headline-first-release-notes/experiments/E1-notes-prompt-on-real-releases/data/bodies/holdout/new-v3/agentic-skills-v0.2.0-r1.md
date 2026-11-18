A new Go plugin brings eight opinionated Go skills, and the Laravel skills were rewritten around clearer rules and one pattern per job.

✨ Added
- Go plugin with eight skills covering architecture, actions, types, interfaces, errors, concurrency, testing and quality, installable with `npx agntc@latest add leeovery/agentic-skills/go`.
- Laravel multi-tenancy guidance for stancl/tenancy v4, including a tenancy-aware test harness that reuses one testing tenant.
- Laravel package guidance for building a package with a manager, drivers, a shipped fake and Testbench tests.
- Laravel quality setup with a house `pint.json` and a Rector config you can copy into a project.
- Laravel services guidance with a complete Saloon driver example covering config, connector, requests, null driver and tests.
- Laravel value-object guidance for money using moneyphp/money, with a factory and cast.
- Nuxt gains 3 skills (18 in total), including a new server-data skill covering Drizzle ORM, Cloudflare D1 and NuxtHub.

🔧 Changed
- Laravel skills now give explicit rules per pattern, and each says when to reach for situational techniques.
- Laravel architecture follows the project's existing layout, with flat `app/` for new projects and one route file per surface.
- Laravel testing uses `test()` with will/can names, container mocks, scoped fakes and factory states.
- Laravel validation Form Requests only validate, with no `authorize()` or `toDto()`, and are tested with endpoint datasets.
- Laravel DTO guidance maps data explicitly through typed request helpers and a transformer per surface.
- Laravel exceptions now carry their own status code and reporting rule through the `Httpable` concern.
- Laravel jobs, listeners and state machines are thin wrappers that call one action and are named after it.
- The release script now runs `mint release` and requires `mint` to be installed.

🗑️ Removed
- Laravel reference files that were folded into the rewritten skills, including the architecture checklist and examples, query-object docs and validation macro setup.
