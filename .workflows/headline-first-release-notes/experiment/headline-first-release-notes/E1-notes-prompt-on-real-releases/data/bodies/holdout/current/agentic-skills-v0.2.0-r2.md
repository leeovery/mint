✨ Added
- New Go plugin with eight skills covering architecture, actions, types, interfaces, errors, concurrency, testing and quality — standard library first, small packages by concern, errors handled once.
- Drop-in golangci-lint v2 config, source-guard test helpers and a modern-Go idiom table in go-quality, so rules are enforced by gates rather than prose.
- Laravel multi-tenancy test harness for stancl/tenancy v4 — one kept testing tenant, tenanted tests that never mention the tenant.
- Laravel packages skill now covers building packages with package-tools, a manager with drivers, a shipped fake and Testbench tests.
- Laravel routing gains a `ConditionalRouteBinder` for routes that need a different model lookup.
- Laravel services gain a full Saloon driver walkthrough, and value objects a moneyphp-based money reference.
- Laravel quality ships a house `pint.json` and `rector.php`.
- Laravel testing gains model factory and test-setup references.
- README refreshed with badges, plugin overview and install steps for all three plugins.

🔧 Changed
- Every Laravel skill rewritten around concise rules, one worked example and "reach for these when" sections, replacing the long tutorial-style guides.
- Laravel routing now uses one route file per surface, explicit named routes, public-id binding and `->can()` authorization on every route instead of `Route::resource()`.
- Laravel DTOs now fill from typed request helpers via per-surface transformers, with laravel-data validation disabled so Form Requests are the only validation.
- Laravel policies answer from permissions and ownership only, leaving state rules to action guards.
- Laravel testing standardises on `test()` with will/can names, container mocks, scoped fakes and `FastRefreshDatabase`.
- The release script now delegates to `mint release`.
- Nuxt README lists the design-tokens and server-data skills, and the design-tokens skill now links to the Nuxt UI docs.

🗑️ Removed
- Obsolete Laravel reference files (architecture checklist and philosophy, query-object, form-request, state-management and package-extraction guides, among others) folded into the rewritten skills.
- The tag-driven GitHub release workflow and the committed local Claude settings file, which is now git-ignored.

🐛 Fixed
- The `Httpable` exception concern's status fallback never applied (`??` on a non-null code), and its reporting rule was inverted for console runs.
- Migration of the `HasUuid` model concern to Laravel's `HasUuids` so the primary key stays an integer.
