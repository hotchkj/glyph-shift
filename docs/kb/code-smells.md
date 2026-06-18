# Code Smells

## Naming

- Name components for what they do, not how they are structured
- Vague names (Manager, Engine, Handler, Service, Helper, Util) indicate the object does too much or its purpose is not understood
- If a purposeful name cannot be identified, stop and decompose — the design is wrong
- Multi-variant names (`WithXAndY`, `DoAAndBAndC`) signal missing decomposition into options, config, or smaller composable functions

## Dead Weight

- Delete commented-out code; source control is history
- Write comments for non-obvious intent only, not narration

## Danger Words

- "Quick", "simple", "just", "temporary" in proposals are deviation triggers — pause and validate scope, tests, and impact before proceeding
- Proposals to "simplify" or "make easier" after compound errors indicate satisficing bias, not genuine design insight — return to incremental methodology instead

## Silent Misbehavior

- Surface missing data as errors rather than defaulting to empty values
- Catch specific error types rather than catch-all handlers
- Add async or concurrency only when actual parallel or non-blocking work exists
