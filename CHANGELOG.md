# Changelog
All notable changes to this project will be documented in this file. See [conventional commits](https://www.conventionalcommits.org/) for commit guidelines.

- - -
## v0.3.0 - 2026-09-23
#### Features
- (**cli**) add yass update to replace the binary with the newest release - (411388a) - Claude, *Claude Fable 5*
- (**install**) add curl-able installer and Homebrew distribution - (75b46d8) - Claude, *Claude Fable 5*
#### Documentation
- document the installation and update paths - (4fa29ed) - Claude, *Claude Fable 5*
- add semantic agent navigation indexes - (caacd15) - Jacob Alheid
#### Refactoring
- simplify installer, formula staging, and update tests - (4875678) - Claude, *Claude Fable 5*

- - -

## v0.2.0 - 2026-09-16
#### Features
- (**cli**) orient a cold reader from bare invocation - (d1a845a) - Jacob Alheid
- (**cli**) implement the yass CLI from its spec set - (899fc45) - Jacob Alheid
- (**cli**) spec an agent-facing yass CLI merging three experiment branches - (23b64c3) - Jacob Alheid
#### Bug Fixes
- (**cli**) exempt the root-path line from the overview width test - (09fd3c1) - Jacob Alheid, *Devin*
#### Documentation
- (**cli**) fence the build prompt's reading scope to the spec set - (e5f26e4) - Jacob Alheid
- (**cli**) add build-prompt for implementing the CLI from spec - (004c184) - Jacob Alheid
- describe the orientation entry point and the carried corpus - (ff5c2a7) - Jacob Alheid
- add CLAUDE.md reading policy and CLI status note - (4ef864a) - Jacob Alheid
- add root CLAUDE.md with reading policy and CLI build notes - (82e447d) - Jacob Alheid
#### Build system
- (**cli**) ship release archives via cog pre_bump_hook - (5535e6f) - Jacob Alheid, *Devin*
#### Miscellaneous Chores
- drop leftover experiment CLAUDE.md - (b7425d0) - Jacob Alheid, *Devin*

- - -

## v0.1.0 - 2026-09-16
#### Features
- (**lang**) define the project root and require root.yass.yaml - (063bc5c) - Jacob Alheid
- (**lang**) draft design-block document type + relation partition (unadopted) - (1e8aa90) - Jacob Alheid
- (**script**) add cursor-agent wrapper for spec-following experiment - (f788380) - Jacob Alheid
#### Bug Fixes
- (**lang**) state the preamble key closure in the language definition - (87613f2) - Jacob Alheid
- (**lang**) reference the five per-slot specs from Slot - (14b4f5f) - Jacob Alheid
- (**lang**) redefine SEE on the resolution axis; no fourth relation - (76bd8e7) - Jacob Alheid
#### Documentation
- (**context**) added index file - (c5908f8) - Jacob Alheid
- (**context**) add DESIGN-BLOCKS proposal — design: document type, repurposed USES - (f7f1137) - Jacob Alheid
- (**context,experiment**) restructure documentation to stop confusing the AI - (2cfe1a6) - Jacob Alheid
- (**design-blocks**) update status — drafted into language via PR #16 - (afb6fc2) - Jacob Alheid
- (**experiment**) move out old prompts - (11d98b3) - Jacob Alheid
- (**guidance**) drop the parked test-taxonomy section - (510b09d) - Jacob Alheid
- (**guidance**) cut restated reachability rule and invented example - (1b4dc2b) - Jacob Alheid
- (**guidance**) drop the holding-doc disclaimer - (5fafaee) - Jacob Alheid
- (**guidance**) adopt the guiding principle into GUIDANCE - (ed2ea3d) - Jacob Alheid
- stop recording implementation status in the language reference - (1ebab7e) - Jacob Alheid
- scope GOAL.md and experiment docs as non-authoritative - (a690043) - Jacob Alheid
- add spec authoring guidance and format change recommendations - (422974d) - Jacob Alheid
- propose man-page alignment of v1 keyword vocabulary - (4053192) - Jacob Alheid
- capture triage notes; drop dangling DECISIONS.md refs - (67a20f1) - Jacob Alheid
- add implementation ordering guidance for spec documents - (0f07576) - Jacob Alheid
- add cross-implementation feedback from 7-language CLI experiment - (a0bd492) - Jacob Alheid
#### Miscellaneous Chores
- (**experiment**) remove ephemeral round-06/07/08 artifacts and test-specs - (2acbf10) - Jacob Alheid
- (**experiment**) round-08 crib results — compliance NULL both arms, design-blocks docket complete - (85b5a2e) - Jacob Alheid
- (**experiment**) round-08 crib probe — postgres tech-constraint spec+oracle, committed pre-panel - (210d433) - Jacob Alheid
- (**experiment**) round-07 granary results — clarity NULL, design-blocks wontfix - (032c3d5) - Jacob Alheid
- (**experiment**) round-07 granary probe — algorithm-clarity spec+oracle, committed pre-panel - (62b68da) - Jacob Alheid
- (**experiment**) round-06 results — design-blocks probe graded, convergence 2/2 - (e0ee39f) - Jacob Alheid
- (**experiment**) round-06 kiln probe — two-arm design-blocks spec+oracle, committed pre-panel - (82971e7) - Jacob Alheid
- (**experiment**) remove example cli spec set, decouple kept refs - (c2c94df) - Jacob Alheid
- (**experiment**) add reusable run-prompt for kicking off future runs - (76a6309) - Jacob Alheid
- (**experiment**) add reusable cold-prompt, de-stale GOAL.md refs - (518ad41) - Jacob Alheid
- (**experiment**) drop stale "Current status" snapshot from GOAL.md - (240c0a5) - Jacob Alheid
- (**experiment**) round-05 scale probe — refute sequencing/override, resolve ref-target, HALT - (38ace1a) - Jacob Alheid
- (**experiment**) round-05 author scale probe spec and oracle - (e21235f) - Jacob Alheid
- (**experiment**) round-04 resolve CONFORMS cluster + new residual-reachability - (20319ca) - Jacob Alheid
- (**experiment**) round-04 author CONFORMS-cluster probe (axle) + oracle - (ea15399) - Jacob Alheid
- (**experiment**) round-03 close — refute structured-obligation cluster, resolve 2 new defects - (0524f1f) - Jacob Alheid
- (**experiment**) round-03 author vault probe spec and oracle - (9a0ea69) - Jacob Alheid
- (**experiment**) round-02 resolve composition cluster (dataflow + cross-cutting) - (9075400) - Jacob Alheid
- (**experiment**) round-02 add apiary pipeline spec set and oracle - (8643612) - Jacob Alheid
- (**experiment**) round-01 fix default-error-policy + input-segmentation - (aa61061) - Jacob Alheid
- (**experiment**) round-01 add tab + CRLF oracle batches - (68212d1) - Jacob Alheid
- (**experiment**) round-01 panel launched, awaiting grading - (4148def) - Jacob Alheid
- (**experiment**) round-01 advance status to panel-run phase - (6bc3c29) - Jacob Alheid
- (**experiment**) round-01 add berth probe spec and oracle - (2219688) - Jacob Alheid
- (**experiment**) add round-01 harness smoke spec, oracle, and cold prompt - (de0cd62) - Jacob Alheid
- (**experiment**) plan round-01 targeting structured-obligation cluster - (122b299) - Jacob Alheid
- (**experiment**) add GOAL.md north star and experiment scaffolding - (1f05aed) - Jacob Alheid
- add root.yass.yaml for this repository - (b678b2f) - Jacob Alheid

- - -

## v0.0.4 - 2026-06-07
#### Bug Fixes
- (**spec**) correct broken CONFORMS refs and add follow-up notes - (868112e) - Jacob Alheid
#### Miscellaneous Chores
- add script/.gitignore - (4a26d78) - Jacob Alheid

- - -

## v0.0.3 - 2026-06-06
#### Bug Fixes
- (**cli**) pin list truncation, sorting, normalization, parse-failure exit - (fd79f46) - Jacob Alheid
- (**cli**) pin query multi-match dispatch, CONFORMS inlining, output emitter - (64a4e6d) - Jacob Alheid
- (**cli**) pin validate ordering, counting, exhaustiveness, edge cases - (dcbadb0) - Jacob Alheid
- (**cli**) tighten FindProjectRoot precedence, DiscoverSpecFiles; add ExpandGlob - (5386bfe) - Jacob Alheid
- (**cli**) pin ExitCode classification, ErrorLine format, Dispatch invariants - (0b20894) - Jacob Alheid
- (**cli**) add machine-stable error-code table - (0ab05b0) - Jacob Alheid
- (**meta**) pin RefTarget grammar, name composition, YAML dialect - (db6fae3) - Jacob Alheid
- (**schema**) broaden refTarget pattern to support project-root paths - (f0a18e4) - Jacob Alheid
#### Miscellaneous Chores
- (**gitignore**) ignore scheduled_tasks.lock - (8d62d02) - Jacob Alheid

- - -

## v0.0.2 - 2026-06-05
#### Bug Fixes
- resolve drift across context docs against spec sources - (fd2d4dd) - Jacob Alheid
- drop RFC 2119 synonyms from normativity vocabulary - (e7bb435) - Jacob Alheid
#### Miscellaneous Chores
- (**gitignore**) ignored worktrees - (1af03fe) - Jacob Alheid

- - -

Changelog generated by [cocogitto](https://github.com/cocogitto/cocogitto).