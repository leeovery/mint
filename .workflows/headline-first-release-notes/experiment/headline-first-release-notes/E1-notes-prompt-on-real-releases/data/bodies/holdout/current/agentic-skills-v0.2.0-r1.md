✨ Added
- New Go plugin with eight skills — architecture, actions, types, interfaces, errors, concurrency, testing and quality, covering standard-library-first Go, named types, errors handled once and a golangci-lint v2 config.
- Laravel multi-tenancy skill now covers stancl/tenancy v4, including tenant identification, bootstrappers, provisioning and a ready-made tenant test harness.
- Laravel services skill gains a full Saloon driver walkthrough, covering config, connector, requests, null driver and driver tests.
- Laravel packages skill now covers building a package with package-tools, a manager with drivers, a shipped fake and Testbench tests.
- Laravel quality skill ships a house Pint config and a Rector config, plus a debug-call architecture test.
- Laravel routing skill adds a ConditionalRouteBinder reference for routes that need different model lookups.
- Laravel controllers skill adds a reference for base controller response helpers, API resources and the date transformer.
- Laravel testing skill adds references for model factories and test setup.
- Nuxt server-data skill is now listed in the Nuxt plugin README, alongside the design-tokens skill.

🔧 Changed
- Every Laravel skill rewritten to a tighter rules-and-reasons format with consistent examples, anti-patterns and related skills.
- Laravel controllers now use singular `{Model}Controller` names, invokable controllers for non-resource operations, and no return types on controller methods.
- Laravel routing now uses one route file per surface, explicit named routes with singular kebab-case names, and public-id binding instead of `Route::resource()`.
- Laravel DTOs are now filled through explicit typed mapping, usually a transformer per surface, with laravel-data validation switched off.
- Laravel state machines now use `updateTo{State}()` and `is{State}()` helpers instead of `markAs*` and `canBe*`.
- Laravel testing now uses `test()` with will/can names, container mocks and endpoint feature tests on a real database.
- Laravel policies now answer from permissions and ownership only, leaving state rules to action guards.
- Laravel architecture now follows the project's existing layout, with flat `app/` for new projects.
- Laravel value objects now use `readonly` classes with `Castable` casts, and money is handled with moneyphp.
- Root and per-plugin READMEs redesigned with badges, a "How a Skill Loads" section and updated skill tables.
- The `release` script now delegates to `mint release`.

🗑️ Removed
- Reference files from Laravel skills whose content moved into the main skill files, including architecture, query-objects, jobs-listeners, query-builders, route-binding, form-requests and the old PHP examples.
- GitHub release workflow, which built releases from tag annotations.
- Committed Claude local settings file, which is now git-ignored.

🐛 Fixed
- Laravel exceptions `Httpable` fallback now actually returns 500 for a missing code and no longer reports client errors during requests.
- Nuxt design-tokens skill no longer links to a non-existent nuxt-ui skill and points to the Nuxt UI docs instead.
