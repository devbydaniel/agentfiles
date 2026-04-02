# Changelog

## [0.4.0](https://github.com/devbydaniel/agentfiles/compare/v0.3.0...v0.4.0) (2026-03-25)


### Features

* add codex layout (AGENTS.md + .agents/ skills) ([df45f1e](https://github.com/devbydaniel/agentfiles/commit/df45f1e82567b0e94947a88f23697c9a100d78c5))
* add exec_args config field for default af exec arguments ([15b514c](https://github.com/devbydaniel/agentfiles/commit/15b514c4dd147269588ab3d9db19f98073ec7bbc))
* add grouped skills ([6856605](https://github.com/devbydaniel/agentfiles/commit/6856605e36aac2e8b1193eb8645703757452017b))
* add pi extensions ([286ce29](https://github.com/devbydaniel/agentfiles/commit/286ce29441952c8413dd5b5998aed2533073826b))
* add subagent support (store, deploy, push, CLI) ([b639194](https://github.com/devbydaniel/agentfiles/commit/b639194fbc82742313b823e346c3053f1cfd5651))
* support store field in manifest for multi-store repos ([c15b5de](https://github.com/devbydaniel/agentfiles/commit/c15b5de3789a309f29fc427b227114a8fbaf10a1))


### Bug Fixes

* fix cursor AGENTS.md ([8d65eb7](https://github.com/devbydaniel/agentfiles/commit/8d65eb7944214de5e3b224e2dcb6a82a97b6215a))

## [0.3.0](https://github.com/devbydaniel/agentfiles/compare/v0.2.0...v0.3.0) (2026-03-10)


### Features

* add central registry with apply-all command ([fee4a74](https://github.com/devbydaniel/agentfiles/commit/fee4a74a252dea9810ce78f15f892e2350410c43))
* add config package for multi-store support and store provenance in lock entries ([a22eaec](https://github.com/devbydaniel/agentfiles/commit/a22eaeca994a459e17513e8030dd88040d411fbd))
* add exec command to launch agent CLI for registered repos ([0463536](https://github.com/devbydaniel/agentfiles/commit/046353635bb75a045b35b45cbf6c5d2a72d27755))
* move registry to config with repos, local merge, and LoadFromConfig ([06a6e16](https://github.com/devbydaniel/agentfiles/commit/06a6e163c4436d9c4d334b1e286af21a879df474))
* multi-store apply and manifest resolution with store-qualified assets ([1f53089](https://github.com/devbydaniel/agentfiles/commit/1f530898044593219f2db4036a9c45cc26f135b1))
* multi-store integration test and documentation ([3b8c2b9](https://github.com/devbydaniel/agentfiles/commit/3b8c2b9b275dec6f981ff8730d661dd0e195fa6c))
* prune stale assets on apply ([3e1ac81](https://github.com/devbydaniel/agentfiles/commit/3e1ac817b8f3d6e0fcd6ca1cef6ef4fa3354ff6b))
* update all CLI commands to use config-based multi-store ([ef4ec93](https://github.com/devbydaniel/agentfiles/commit/ef4ec93aaaf9c9784d4bdbad5eb8000acd61cc2d))
* update diff and status commands for multi-store support ([190326c](https://github.com/devbydaniel/agentfiles/commit/190326c3b2b9bf982ae801254f2250eb8a51b00c))
* update push to route changes to correct store via lock entry provenance ([52744f6](https://github.com/devbydaniel/agentfiles/commit/52744f605a12de3349a1e597794e30d44901e275))
* user-level agent file deployment ([bc92b6b](https://github.com/devbydaniel/agentfiles/commit/bc92b6bd4475fa21c64f1ecc0b9c9f2af1f1644d))


### Bug Fixes

* always record assets in lock even when skipped ([9d6cd1c](https://github.com/devbydaniel/agentfiles/commit/9d6cd1c094528d97f2d83001613181a943f2bff7))
* fix user scoped skill location for pi ([bd4f790](https://github.com/devbydaniel/agentfiles/commit/bd4f790e5c9838d0005241e7bdadcc81c8956467))
* goimports formatting ([2afa828](https://github.com/devbydaniel/agentfiles/commit/2afa8282eb502ef10aa99d7a2f5b07a9eaef3390))

## [0.2.0](https://github.com/devbydaniel/agentfiles/compare/v0.1.0...v0.2.0) (2026-03-09)


### Features

* add commands for skill, agent, plugin, resource (step 5) ([81fa8a7](https://github.com/devbydaniel/agentfiles/commit/81fa8a7e3a88226aeb4d59acbd0661e92d9fa296))
* apply command (step 7) ([e1bba37](https://github.com/devbydaniel/agentfiles/commit/e1bba3776c070795ef3e326540162641b94b2314))
* init command (step 9) ([98eeb3f](https://github.com/devbydaniel/agentfiles/commit/98eeb3fc3ac0b95c97296572fbb33dbdbca6c23e))
* integration test and README (step 11) ([43f2f91](https://github.com/devbydaniel/agentfiles/commit/43f2f9194e44a7fd5a1248fe82cd51cc73110ab1))
* layout engine (step 4) ([dc914e6](https://github.com/devbydaniel/agentfiles/commit/dc914e63eb14b96886d120336d51d4b1dfe67fad))
* list, diff, status commands (step 10) ([5540d0e](https://github.com/devbydaniel/agentfiles/commit/5540d0e81314b70261cbdf82f2be06e102a98ec9))
* lock file management (step 6) ([de48740](https://github.com/devbydaniel/agentfiles/commit/de487409a123b77135b0b4c2d8d992910a7c820b))
* manifest and bundle parsing (step 3) ([1ae2f87](https://github.com/devbydaniel/agentfiles/commit/1ae2f878348cdf25f08105535c4f3bf1b5940fb8))
* project scaffolding and CLI skeleton (step 1) ([815ffaa](https://github.com/devbydaniel/agentfiles/commit/815ffaa901b736d45e3d883be87ffed70f89d30b))
* push command (step 8) ([d04dda2](https://github.com/devbydaniel/agentfiles/commit/d04dda23cf28f9588a12cbe18c59e3cf0a99a680))
* source store model and init-store command (step 2) ([9d1da19](https://github.com/devbydaniel/agentfiles/commit/9d1da19c568a43966d376243c35b69b52f7b4b08))
