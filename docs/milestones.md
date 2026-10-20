# Milestones

GitHub **project milestones** on the remote are currently empty. This file is the source of truth.

## Strategic track (M1–M5)

| ID | Status | Focus |
|----|--------|--------|
| **M1** | **Done** | Tracked migrations (001–032), demo SSH assets, `docs/openapi.yaml` |
| **M2** | **Done** | Bearer PAT middleware + scope enforcement + contract tests |
| **M3** | Next | Production SSH shop live (deploy + Stripe + authorized_keys) |
| **M4** | Next | Hardening: HMAC fail-closed, money-path tests, DIAN provider |
| **M5** | Next | Product depth: cities, draft orders, split mega-packages |

## Shipped (MVP)

SSH TUI shop, Stripe Checkout + webhooks/reconcile, inventory reservations, outbox→NATS fulfillment, admin RBAC, newsletter double-opt-in, Compose overlays, CI/govulncheck, observability basics, **M1 clone completeness**, **M2 Bearer PAT auth**.

## Engineering backlog

Concrete improvements mined from the codebase (handlers, SQL, TUI, Stripe, Compose, CI, compliance). **956 items.** IDs are stable for GitHub issue import (milestone-backlog-NNNN).

| ID | Category | Slug | Improvement |
|----|----------|------|-------------|
| B0100 | admin | `anomaly-detection` | Anomaly detection on refunds |
| B0101 | admin | `audit-events-filter-ui` | Filter UI for audit events |
| B0102 | admin | `bulk-email-receipt` | Resend receipts in bulk |
| B0103 | admin | `bulk-order-status` | Bulk status transition tool |
| B0104 | admin | `change-freeze` | Change freeze calendar integration |
| B0105 | admin | `console-csp` | Tighten CSP stop third-party JS |
| B0106 | admin | `console-csrf` | CSRF for admin console |
| B0107 | admin | `console-dark-theme` | Readable dark theme for console |
| B0108 | admin | `console-keyboard-shortcuts` | Keyboard shortcuts in admin console |
| B0109 | admin | `console-spa-split` | Move console HTML to static+CSP |
| B0110 | admin | `dead-letter-payload-preview` | Redacted DLQ payload preview |
| B0111 | admin | `dead-letter-replay-idempotent` | Safe double-submit replay |
| B0112 | admin | `diff-view-prices` | Diff view for price changes |
| B0113 | admin | `dual-control-refunds` | Two-person rule large refunds |
| B0114 | admin | `heatmap-orders` | Order heatmap by city |
| B0115 | admin | `image-virus-scan` | ClamAV before MinIO |
| B0116 | admin | `impersonate-readonly` | Read-only customer impersonation |
| B0117 | admin | `inventory-adjust-reason` | Structured inventory adjust reasons |
| B0118 | admin | `invima-bulk-import` | CSV import with INVIMA validation |
| B0119 | admin | `invoice-retry-ui` | UI for invoice retries |
| B0120 | admin | `keyboard-first-ops` | Keyboard-first ops console |
| B0121 | admin | `orders-export-streaming` | Stream CSV exports |
| B0122 | admin | `orders-filter-status-multi` | Multi-status filter on admin orders |
| B0123 | admin | `pii-redaction-export` | Gate PII in exports by role |
| B0124 | admin | `price-change-outbox` | Emit catalog.price_changed |
| B0125 | admin | `price-history-pagination-cursor` | Cursor paginate price history |
| B0126 | admin | `product-image-virus-scan` | Virus scan before MinIO put |
| B0127 | admin | `rbac-export-audit` | Finance+reason for export |
| B0128 | admin | `rbac-finance-only-refunds` | Finance role for money-out |
| B0129 | admin | `refund-feature-flag` | Gate refunds behind flag |
| B0130 | admin | `saved-views` | Saved filter views in console |
| B0131 | admin | `shipment-transition-email-flag` | Show email flag in console |
| B0132 | admin | `sla-timers` | SLA timers on open orders |
| B0133 | admin | `sql-explorer-forbid` | Forbid raw SQL explorer in console |
| B0134 | admin | `status-override-feature-flag` | Gate status override default-off |
| B0135 | api | `address-delete-etag` | If-Match on DELETE addresses |
| B0136 | api | `addresses-write-idempotency` | Idempotency on POST /addresses |
| B0137 | api | `admin-customers-lookup` | Admin lookup customer by fingerprint |
| B0138 | api | `admin-customers-pii-gate` | PII gate on admin customer views |
| B0139 | api | `admin-dead-letter-rl` | adminRL on Stripe DLQ handlers |
| B0140 | api | `admin-image-idempotency` | Content-hash idempotent image upload |
| B0141 | api | `admin-inventory-bulk` | Bulk inventory adjust endpoint |
| B0142 | api | `admin-invoice-retry` | Idempotent DIAN re-issue |
| B0143 | api | `admin-orders-list-rbac` | Consistent requireAdminRole wraps |
| B0144 | api | `admin-orders-missing-rl` | Put admin list/detail/export behind adminRL |
| B0145 | api | `admin-products-image-get` | GET product image metadata admin |
| B0146 | api | `admin-products-soft-delete` | Soft-delete products instead of hard DELETE |
| B0147 | api | `batch-catalog-get` | Batch get products by ids |
| B0148 | api | `bulk-cancel-admin` | Admin bulk cancel pending |
| B0149 | api | `cart-address-idempotency` | Idempotency on PUT /cart/address |
| B0150 | api | `cart-currency-lock` | Lock cart currency to COP once first line added |
| B0151 | api | `cart-max-lines` | Enforce max cart lines and max qty per SKU |
| B0152 | api | `cart-payment-idempotency` | Idempotency on PUT /cart/payment-method |
| B0153 | api | `cart-put-idempotency` | Idempotency-Key on PUT /cart and items |
| B0154 | api | `catalog-abv-filter-index` | Index products for abv_min/abv_max filters |
| B0155 | api | `catalog-etag-if-none-match` | Consistent ETag/If-None-Match on catalog GETs |
| B0156 | api | `catalog-etag-weak` | Weak ETags ignoring volatile qty |
| B0157 | api | `catalog-featured-cache` | Cache featured catalog separately from full list |
| B0158 | api | `catalog-pagination-cursor` | Stable cursors for large catalogs |
| B0159 | api | `checkout-url-allowlist` | Allowlist success/cancel URLs |
| B0160 | api | `conditional-put-cart` | If-Match on PUT /cart |
| B0161 | api | `cors-policy-explicit` | CORS allowlist if admin browser grows |
| B0162 | api | `customer-cancel-reason` | Persist cancel reason to audit |
| B0163 | api | `deprecation-headers` | Deprecation headers for legacy routes |
| B0164 | api | `docs-redoc-auth-cookie` | Optional short-lived cookie for Redoc after bearer |
| B0165 | api | `email-confirm-error-codes` | Distinct expired/used/unknown codes |
| B0166 | api | `email-confirm-rl` | Rate-limit confirm by token+IP |
| B0167 | api | `email-unsubscribe-idempotency` | already_unsubscribed code |
| B0168 | api | `endpoints-md-sync` | CI-diff docs/endpoints.md vs ImplementedRoutes |
| B0169 | api | `error-details-array` | Optional details[] for multi-field errors |
| B0170 | api | `field-mask-patch` | Field masks for PATCH APIs |
| B0171 | api | `graphql-not-planned` | Document GraphQL explicitly out of scope |
| B0172 | api | `head-catalog` | HEAD /catalog for cache revalidate |
| B0173 | api | `healthz-version-build` | Include git SHA in healthz/readyz |
| B0174 | api | `hmac-missing-headers` | Structured HMAC failure codes |
| B0175 | api | `idempotency-body-mismatch-412` | Conflict when key reused with different body SHA |
| B0176 | api | `idempotency-key-cleanup-job` | Purge expired http_idempotency_keys |
| B0177 | api | `json-merge-patch` | Support JSON Merge Patch |
| B0178 | api | `metrics-auth-optional` | Optional bearer/metrics scrape auth |
| B0179 | api | `metrics-path-cardinality` | Normalize path labels in metrics |
| B0180 | api | `oauth-authorize-idempotency` | Idempotent authorize code issuance |
| B0181 | api | `oauth-token-rl` | Rate-limit POST /oauth/token |
| B0182 | api | `openapi-admin-image-upload` | Document multipart product image upload |
| B0183 | api | `openapi-error-envelope` | Reusable Error component for all 4xx/5xx |
| B0184 | api | `openapi-error-schema` | Shared Error schema for generators |
| B0185 | api | `openapi-pagination-params` | Document pagination consistently |
| B0186 | api | `openapi-request-bodies` | RequestBody schemas for mutations |
| B0187 | api | `openapi-request-response-schemas` | Expand OpenAPI beyond 200 stubs |
| B0188 | api | `openapi-securitySchemes` | Document HMAC and Bearer schemes |
| B0189 | api | `order-cancel-status-codes` | Dedicated cancel conflict codes |
| B0190 | api | `order-display-ref-format` | Document SH- display_ref format and year rollover |
| B0191 | api | `order-get-authz` | 404 not 403 for other customers refs |
| B0192 | api | `orders-list-cursor` | Cursor pagination for orders |
| B0193 | api | `orders-list-filters` | Status/date filters on GET /orders |
| B0194 | api | `payment-methods-collect-rl` | Separate RL for SetupIntent collect |
| B0195 | api | `payment-methods-delete-authz` | Verify Stripe PM ownership on delete |
| B0196 | api | `payment-methods-detach-stripe` | Detach Stripe PM on DELETE |
| B0197 | api | `profile-patch-validation` | Field-level validation error codes |
| B0198 | api | `profile-put-vs-patch-semantics` | Document PUT replace vs PATCH partial |
| B0199 | api | `public-catalog-cdn` | CDN cache public catalog |
| B0200 | api | `put-profile-idempotency` | Idempotent profile PUT/PATCH |
| B0201 | api | `rate-limit-by-sku` | RL aggressive SKUs separately |
| B0202 | api | `rate-limit-headers` | Emit Retry-After and X-RateLimit-* |
| B0203 | api | `readyz-stripe-optional` | Optional Stripe probe when not mock |
| B0204 | api | `search-suggest` | Catalog suggest endpoint |
| B0205 | api | `shipping-estimate-openapi` | Specify shipping estimate inputs/outputs |
| B0206 | api | `shipping-estimate-rl` | Rate-limit shipping estimate |
| B0207 | api | `shipping-estimate-validation` | Reject unsupported cities with hints |
| B0208 | api | `subscription-cancel-endpoint-docs` | Document cancel semantics in OpenAPI |
| B0209 | api | `subscription-cancel-idempotency` | Idempotent subscription cancel |
| B0210 | api | `subscription-list-authz` | Prevent customer_id spoofing on list |
| B0211 | api | `subscription-update-validation` | Field errors on subscription PUT |
| B0212 | api | `tokens-create-idempotency` | Idempotent POST /tokens |
| B0213 | api | `tokens-list-pagination` | has_more on GET /tokens |
| B0214 | api | `view-init-partial-errors` | Per-section errors in view/init |
| B0215 | api | `view-init-partial-failure-codes` | Stabilize viewInitWarning section codes |
| B0216 | api | `webhook-max-bytes-metric` | Metric when webhook hits MaxBytes |
| B0217 | api | `webhooks-outbound` | Outbound webhooks for merchants |
| B0218 | api | `webhook-stripe-path-config-test` | Test STRIPE_WEBHOOK_PATH remount |
| B0219 | auth | `admin-support-cancel-only-pending` | Support role cannot cancel post-paid |
| B0220 | auth | `admin-token-hash-at-rest` | Hash ADMIN_* tokens at rest |
| B0221 | auth | `admin-token-rotation-api` | Audited rotation without redeploy |
| B0222 | auth | `age-gate-replay` | Persist age verification with anti-replay |
| B0223 | auth | `bearer-pat-middleware` | ~~Done (M2)~~ Wire Bearer PAT middleware via tokens.GetByPrefix + hash verify |
| B0224 | auth | `bearer-pat-scope-enforce` | ~~Done (M2)~~ Enforce PAT scopes on cart/orders/profile/subscriptions |
| B0225 | auth | `bearer-vs-hmac-contract-tests` | ~~Done (M2)~~ Contract tests for Bearer success/fail vs HMAC paths |
| B0226 | auth | `break-glass-admin` | Break-glass admin procedure |
| B0227 | auth | `device-attestation-future` | Future device attestation research |
| B0228 | auth | `fingerprint-spoof-hardening` | Reject unsigned fingerprint injection in prod |
| B0229 | auth | `hmac-body-sha-header-required` | Require body SHA header on all signed POSTs |
| B0230 | auth | `hmac-key-rotation-overlap` | Dual-key acceptance window metrics |
| B0231 | auth | `hmac-keys-required-prod` | Refuse start if prod has no HMAC keys |
| B0232 | auth | `hmac-nonce-janitor-job` | Scheduled reap of expired hmac_nonces |
| B0233 | auth | `hmac-nonce-reap-job` | Purge expired hmac_nonces under SSH traffic |
| B0234 | auth | `hmac-v1-deprecate` | Fail closed on HMAC v1 in NAMESPACE=prod |
| B0235 | auth | `ip-allowlist-admin` | IP allowlist for admin tokens |
| B0236 | auth | `key-ceremony-hmac` | HMAC key ceremony procedure |
| B0237 | auth | `key-id-header-required` | Require kid header always in prod |
| B0238 | auth | `mfa-admin-optional` | Optional MFA for admin tokens |
| B0239 | auth | `nonce-unique-constraint-verify` | Verify unique(fingerprint,nonce,ts) migration |
| B0240 | auth | `oauth-access-token-usable` | Make OAuth-issued PATs work on customer APIs |
| B0241 | auth | `oauth-client-fk-on-pat` | Add oauth_client_id FK on personal_access_tokens |
| B0242 | auth | `oauth-client-id-prefix-oci` | Enforce oci_ prefix in DB CHECK for client_id |
| B0243 | auth | `oauth-code-ttl-metric` | Metric for unused expired auth codes |
| B0244 | auth | `oauth-pkce` | Add PKCE to OAuth authorize/token |
| B0245 | auth | `oauth-pkce-required` | Require PKCE for public clients |
| B0246 | auth | `oauth-refresh-tokens` | Persist and rotate OAuth refresh tokens |
| B0247 | auth | `oauth-token-revoke-endpoint` | RFC token revocation for OAuth PATs |
| B0248 | auth | `passkey-not-ssh` | Document passkeys N/A for SSH shop |
| B0249 | auth | `pat-expiry-default` | Default expires_at on user-minted PATs |
| B0250 | auth | `pat-last-used-at-update` | TouchLastUsed on every successful Bearer auth |
| B0251 | auth | `pat-prefix-length-check-32` | Align DB CHECK token_prefix length to 32 hex |
| B0252 | auth | `pat-scope-enforcement` | Enforce scopes on mutating routes |
| B0253 | auth | `scim-not-planned` | Document SCIM out of scope |
| B0254 | auth | `session-fixation-n-a` | Document session fixation N/A SSH |
| B0255 | auth | `session-ssh-to-api-ttl` | Align SSH session TTL with HMAC skew |
| B0256 | auth | `ssh-fingerprint-binding-on-bearer` | Decide fingerprint binding when Bearer+SSH headers both present |
| B0257 | auth | `step-up-auth-refunds` | Step-up auth for large refunds |
| B0258 | auth | `support-token-read-only-inventory` | Support can read inventory not write |
| B0259 | auth | `token-binding-dpop` | Research DPoP for PATs |
| B0260 | auth | `workload-identity` | Workload identity for jobs to API |
| B0261 | ci | `action-pinning-sha` | Pin Actions to commit SHAs |
| B0262 | ci | `artifact-retention` | Define CI artifact retention |
| B0263 | ci | `auto-approve-deps` | Auto-approve low-risk Dependabot |
| B0264 | ci | `auto-label-pr` | Auto-label PRs by path |
| B0265 | ci | `commitlint` | Conventional commits lint on PR |
| B0266 | ci | `compose-config-validate` | compose config in CI |
| B0267 | ci | `core-api-unit-job-split` | Non-DB core-api in unit job |
| B0268 | ci | `coverage-gate-money-paths` | Coverage gate on money packages |
| B0269 | ci | `dependabot-gomod` | Dependabot for Go modules |
| B0270 | ci | `diff-coverage` | Diff coverage on PR |
| B0271 | ci | `docker-build-cache` | Cache Go builds in CI |
| B0272 | ci | `e2e-ssh-tui-smoke` | Non-interactive SSH smoke |
| B0273 | ci | `flaky-test-quarantine` | Quarantine flaky tests with issue link |
| B0274 | ci | `golangci-lint` | Optional golangci-lint config |
| B0275 | ci | `go-test-shard` | Shard core-api tests |
| B0276 | ci | `govulncheck` | govulncheck in lint already—keep ADR |
| B0277 | ci | `license-allowlist` | Allowed licenses allowlist |
| B0278 | ci | `license-check` | License compliance check for deps |
| B0279 | ci | `merge-queue` | GitHub merge queue |
| B0280 | ci | `migration-down-optional` | Require down or NO_DOWN note |
| B0281 | ci | `migration-linearity-windows` | Document Git Bash on Windows |
| B0282 | ci | `mutation-testing` | Optional mutation testing money pkgs |
| B0283 | ci | `nightly-full-suite` | Nightly full suite including load |
| B0284 | ci | `openapi-drift-in-lint` | OpenAPI drift in lint job |
| B0285 | ci | `openapi-spectral` | Spectral lint OpenAPI |
| B0286 | ci | `ps1-shellcheck-analogue` | PSScriptAnalyzer parallel shellcheck |
| B0287 | ci | `race-integration-shard` | Shard race integration tests |
| B0288 | ci | `remove-ci-dockerfile-stubs` | Fail if migrations missing |
| B0289 | ci | `required-reviewers` | CODEOWNERS for money paths |
| B0290 | ci | `required-status-checks` | Branch protection required checks |
| B0291 | ci | `sbom-on-docker-build` | SBOM on docker build |
| B0292 | ci | `secret-scan` | gitleaks on PRs |
| B0293 | ci | `shellcheck-powershell` | PSScriptAnalyzer when ps1 added |
| B0294 | ci | `staticcheck-core-api-only-matrix` | Faster staticcheck matrix |
| B0295 | ci | `supply-chain-slsa` | SLSA provenance for images |
| B0296 | ci | `windows-ps-scripts-check` | Syntax-check PowerShell scripts |
| B0297 | compliance | `age-verification-retention` | Purge age_verifications by policy |
| B0298 | compliance | `alcohol-hours-restriction` | Municipal sale-hours gate |
| B0299 | compliance | `alcohol-license-number` | Show commerce license on receipts |
| B0300 | compliance | `audit-events-retention` | Tiered audit retention |
| B0301 | compliance | `auditor-access-mode` | Time-boxed auditor access mode |
| B0302 | compliance | `breach-notification` | Breach notification runbook |
| B0303 | compliance | `consent-purpose-matrix` | Expand consent purposes + UI |
| B0304 | compliance | `consent-versioning` | Version privacy text hash |
| B0305 | compliance | `consent-withdrawal-api` | Withdraw consent API+TUI |
| B0306 | compliance | `cookie-less-tracking-note` | SSH FP as personal identifier |
| B0307 | compliance | `cross-border-transfer` | Cross-border data transfer clauses |
| B0308 | compliance | `customer-data-erasure` | Erasure with fiscal exceptions |
| B0309 | compliance | `customer-data-export` | Ley 1581 data export endpoint |
| B0310 | compliance | `data-map` | Data map systems inventory |
| B0311 | compliance | `data-portability-export` | Customer data portability |
| B0312 | compliance | `data-processing-publish` | Finalize legal data-processing |
| B0313 | compliance | `dian-fail-closed-prod` | Refuse stub DIAN in prod |
| B0314 | compliance | `dian-real-provider` | Real DIAN provider mapping CUFE |
| B0315 | compliance | `dpo-contact` | Publish DPO contact |
| B0316 | compliance | `export-watermark` | Watermark CSV with actor |
| B0317 | compliance | `habeas-data-register` | RNBD registration task |
| B0318 | compliance | `invima-format-regex` | Validate INVIMA reg format on write |
| B0319 | compliance | `invima-required-active` | Bulk activate INVIMA checks |
| B0320 | compliance | `invoice-dian-retention` | DIAN retention vs marketing |
| B0321 | compliance | `legal-hold-flag` | legal_hold blocks erasure |
| B0322 | compliance | `ley1581-customer-erasure-job` | Anonymize PII job |
| B0323 | compliance | `minor-alcohol-warning-persistence` | Warn on welcome/email/checkout |
| B0324 | compliance | `minor-block-hard` | Hard-block underage at age gate |
| B0325 | compliance | `pii-access-report` | Scheduled PII access report |
| B0326 | compliance | `retention-calendar` | Retention calendar automation |
| B0327 | compliance | `rnbd-annual-update` | Annual RNBD update reminder |
| B0328 | compliance | `ropa-record` | Record of processing activities |
| B0329 | compliance | `sic-complaint-channel` | SIC complaint deep link |
| B0330 | compliance | `sic-response-sla` | SLA for SIC complaint responses |
| B0331 | compliance | `tax-breakdown-lines` | IVA/excise on invoices |
| B0332 | compliance | `training-ops-pii` | Ops training for PII handling |
| B0333 | compliance | `vendor-dpa` | DPA template for vendors |
| B0334 | compliance | `vendor-risk-register` | Vendor risk register |
| B0335 | data | `archival-cold-storage` | Archive old orders to cold storage |
| B0336 | data | `audit-events-retention-job` | Purge audit_events per policy |
| B0337 | data | `brin-orders-created` | BRIN index on orders.created_at |
| B0338 | data | `cdc-to-warehouse` | CDC to analytics warehouse |
| B0339 | data | `check-constraints-status` | CHECK constraints all status enums |
| B0340 | data | `concurrent-index-create` | CONCURRENTLY index playbook |
| B0341 | data | `consumer-processed-messages-ttl` | TTL for processed messages |
| B0342 | data | `customers-email-unique-ci` | CI unique email index |
| B0343 | data | `expand-contract-email-cols` | Example expand/contract rename |
| B0344 | data | `fk-on-delete-policy` | Explicit ON DELETE policies |
| B0345 | data | `foreign-data-wrapper` | FDW research for analytics |
| B0346 | data | `http-idempotency-reap` | Reap expired idempotency keys |
| B0347 | data | `invoice-unique-per-order` | One successful invoice per order |
| B0348 | data | `materialized-catalog` | Materialized view for catalog |
| B0349 | data | `migration-033-oauth-pat-fk` | oauth_client_id on PATs |
| B0350 | data | `migration-down-001-032` | Down migrations or notes |
| B0351 | data | `migration-gitignore-clarify` | Clarify migrations are tracked |
| B0352 | data | `nullable-stripe-ids` | CHECK PAID has PI or marker |
| B0353 | data | `null-vs-empty-phone` | Normalize empty string to NULL |
| B0354 | data | `online-ddl` | Online DDL tools evaluation |
| B0355 | data | `orders-partial-indexes` | Partial indexes PENDING_PAYMENT |
| B0356 | data | `orders-status-check` | CHECK orders.status allowlist |
| B0357 | data | `outbox-aggregate-index` | Index outbox by aggregate |
| B0358 | data | `outbox-archive-s3` | Archive SENT outbox to object store |
| B0359 | data | `outbox-partitioning` | Partition outbox_events |
| B0360 | data | `payments-null-provider-ref` | NOT NULL provider_ref when paid |
| B0361 | data | `personal-access-token-name-unique` | Unique token names per customer |
| B0362 | data | `pii-tokenization` | Tokenize PII with vault |
| B0363 | data | `prices-bi-temporal-query` | Optimize price-as-of query plan |
| B0364 | data | `read-replica-routing` | Optional read replica for catalog |
| B0365 | data | `sharding-criteria` | Document when to shard orders |
| B0366 | data | `shipping-cities-expand` | More cities DB-driven allowlist |
| B0367 | data | `shipping-rate-admin-crud` | Admin CRUD for rates/tiers |
| B0368 | data | `shipping-tiers-index` | Index tiers by city+weight |
| B0369 | data | `soft-delete-customers` | Soft-delete + anonymize |
| B0370 | data | `stripe-event-log-partition` | Partition stripe_event_log |
| B0371 | data | `subscriptions-billing-cycle-job` | Renewal job for subscriptions |
| B0372 | data | `temporal-tables` | Research temporal tables for prices |
| B0373 | data | `vacuum-autovacuum-tune` | Tune autovacuum for outbox |
| B0374 | docs | `adr-0020-full` | Write ADR 0020 Terminal.shop parity |
| B0375 | docs | `adr-0021-full-text` | Write ADR 0021 full text |
| B0376 | docs | `adr-index-autogen` | Autogen ADR index from files |
| B0377 | docs | `alert-catalog` | Alert catalog with owners |
| B0378 | docs | `architecture-diagram` | Mermaid architecture in docs/README |
| B0379 | docs | `bearer-pat-howto` | PAT howto for scripts |
| B0380 | docs | `contributor-covenant` | Code of conduct |
| B0381 | docs | `decision-log` | Lightweight decision log |
| B0382 | docs | `dian-ops-runbook` | DIAN ops runbook |
| B0383 | docs | `endpoints-auth-matrix` | Auth matrix per route |
| B0384 | docs | `environment-flag-table-ci` | CI list all flags in docs |
| B0385 | docs | `expand-contract-playbook` | Expand/contract playbook |
| B0386 | docs | `faq-customer` | Customer-facing FAQ content source |
| B0387 | docs | `feature-flags-matrix` | env × flag matrix |
| B0388 | docs | `glossary` | Domain glossary order/reservation/outbox |
| B0389 | docs | `internal-handbook` | Internal engineering handbook |
| B0390 | docs | `milestones-backlog-import` | Import to GitHub issues |
| B0391 | docs | `milestones-github-sync` | Sync GitHub milestones |
| B0392 | docs | `oauth-app-developer-guide` | OAuth developer guide |
| B0393 | docs | `oncall-roster` | On-call roster template |
| B0394 | docs | `openapi-changelog` | OpenAPI version changelog |
| B0395 | docs | `pilot-checklist-150` | Expand roadmap-150 checklist |
| B0396 | docs | `release-notes-process` | Release notes process |
| B0397 | docs | `rfc-process` | RFC process for large changes |
| B0398 | docs | `runbook-index` | Index all runbooks with owners |
| B0399 | docs | `runbook-ley1581-erasure` | Erasure fulfillment steps |
| B0400 | docs | `runbook-refund-stripe` | Stuck refunds runbook |
| B0401 | docs | `runbook-stripe-dlq` | Stripe DLQ triage runbook |
| B0402 | docs | `second-pass-backlog` | Living backlog with owners |
| B0403 | docs | `slo-error-budget` | Checkout/webhook SLOs |
| B0404 | docs | `style-guide-docs` | Docs style guide |
| B0405 | docs | `testing-md-coverage-map` | Map untested packages |
| B0406 | docs | `threat-model` | STRIDE threat model doc |
| B0407 | docs | `tui-md-screens-matrix` | Document every screen |
| B0408 | docs | `video-walkthrough` | Record SSH shop walkthrough |
| B0409 | docs | `windows-dev-guide` | docs/dx/windows.md |
| B0410 | dx | `adr-bodies-checked-in` | Add ADR 0006/0016/0020/0021 files |
| B0411 | dx | `adr-review-sla` | ADR review SLA |
| B0412 | dx | `adr-template` | ADR template markdown |
| B0413 | dx | `api-contract-fixtures` | Generate client fixtures |
| B0414 | dx | `api-fuzz-corpus` | Seed fuzz corpus for handlers |
| B0415 | dx | `api-postman-collection` | Generate Postman from OpenAPI |
| B0416 | dx | `api-seed-idempotent` | Document seed idempotency |
| B0417 | dx | `changelog-automation` | Generate changelog from commits |
| B0418 | dx | `codespaces-devcontainer` | devcontainer.json for Codespaces |
| B0419 | dx | `core-api-air-run` | One-command pwsh migrate+api |
| B0420 | dx | `design-md-public-subset` | Publish non-sensitive DESIGN excerpts |
| B0421 | dx | `dev-reset-windows` | PowerShell dev-up/down/reset |
| B0422 | dx | `direnv-example` | direnv .envrc example |
| B0423 | dx | `editor-launch-json` | Cursor launch configs |
| B0424 | dx | `env-example-complete` | List all feature flags in env examples |
| B0425 | dx | `gitignore-audit-demo-keys` | Verify demo key tracking policy |
| B0426 | dx | `good-first-issues` | Label good first issues from backlog |
| B0427 | dx | `gotestsum-windows` | Colored tests on Windows |
| B0428 | dx | `gotoolchain-env-example` | GOTOOLCHAIN=auto snippet |
| B0429 | dx | `issue-templates` | GitHub issue templates by category |
| B0430 | dx | `local-development-ssh-first` | First-time SSH path docs |
| B0431 | dx | `local-stripe-fixtures` | Checked-in Stripe fixture JSON |
| B0432 | dx | `makefile-or-task` | Taskfile/Makefile wrappers |
| B0433 | dx | `make-windows-targets` | pwsh-friendly targets |
| B0434 | dx | `migrate-down-support` | migrate down/steps |
| B0435 | dx | `migration-path-windows` | Windows MIGRATIONS_PATH |
| B0436 | dx | `onboarding-checklist` | New engineer onboarding checklist |
| B0437 | dx | `pair-programming-guide` | Contributor pair guide |
| B0438 | dx | `powershell-dev-up` | scripts/dev-up.ps1 |
| B0439 | dx | `powershell-stack-up` | scripts/stack-up.ps1 |
| B0440 | dx | `pre-commit-hook` | pre-commit: gofmt vet linearity |
| B0441 | dx | `readme-ps1-section` | Link PowerShell ports |
| B0442 | dx | `readme-quickstart-verify` | 5-minute verify section |
| B0443 | dx | `seed-reset-one-command` | One command seed reset |
| B0444 | dx | `seed-scenarios` | Multiple seed scenarios beyond demo |
| B0445 | dx | `testcontainers-docs` | Document Docker need for tests |
| B0446 | dx | `tui-golden-screenshots` | Bubbletea golden views |
| B0447 | dx | `vscode-settings-gotoolchain` | go.toolsEnvVars docs |
| B0448 | email | `a11y-html-lint` | Accessibility lint for HTML emails |
| B0449 | email | `amp-or-plain-only` | Decide AMP email or plain+HTML only |
| B0450 | email | `bounce-category` | Categorize hard vs soft bounces |
| B0451 | email | `bounce-suppression` | Suppression table before send |
| B0452 | email | `bounce-webhook` | Bounce webhooks mark emails invalid |
| B0453 | email | `click-tracking-off` | Document click tracking off for privacy |
| B0454 | email | `compliance-footer-i18n` | Configurable footer per locale |
| B0455 | email | `confirm-token-hash` | Hash confirmation tokens at rest |
| B0456 | email | `dkim-spf-docs` | DKIM/SPF setup docs for SMTP |
| B0457 | email | `from-name-display` | Configurable From display name |
| B0458 | email | `list-unsubscribe-post` | POST one-click unsubscribe header |
| B0459 | email | `log-provider-json-redact` | Redact addresses in log provider |
| B0460 | email | `payment-link-idempotency` | Dedupe payment link emails |
| B0461 | email | `payment-link-sms-stub` | Optional SMS payment link provider stub |
| B0462 | email | `plain-text-parity` | Ensure HTML/text template parity |
| B0463 | email | `preference-center` | Email preference center page |
| B0464 | email | `receipt-locale` | es_CO primary receipt emails |
| B0465 | email | `receipt-pdf-attachment` | Attach DIAN PDF when present |
| B0466 | email | `send-time-optimize` | Send-time optimization research |
| B0467 | email | `shipping-update-idempotency` | Dedupe shipping update emails |
| B0468 | email | `smtp-provider-hardening` | TLS required pool timeouts |
| B0469 | email | `subscribe-confirm-rate-limit` | Stronger per-IP email subscribe limits |
| B0470 | email | `subscribe-reap-in-compose` | Add email-subscribe-reap to prod |
| B0471 | email | `suppression-ui` | Admin UI for email suppressions |
| B0472 | email | `template-lint-ci` | CI lint HTML email templates |
| B0473 | email | `template-preview-cmd` | CLI render email templates |
| B0474 | email | `template-versioning` | Version email templates |
| B0475 | email | `transactional-only-mode` | Hard disable marketing sends |
| B0476 | email | `unsubscribe-one-click` | RFC 8058 List-Unsubscribe |
| B0477 | fulfillment | `carbon-estimate` | Carbon estimate on shipping |
| B0478 | fulfillment | `carrier-integrations` | Carrier API adapters Colombia |
| B0479 | fulfillment | `carrier-webhook-stub` | Inbound carrier tracking webhook |
| B0480 | fulfillment | `cold-chain-flag` | Cold-chain SKU shipping flag |
| B0481 | fulfillment | `delivered-closes-order` | DELIVERED advances order to SHIPPED/done |
| B0482 | fulfillment | `driver-app-api` | Driver app API research |
| B0483 | fulfillment | `eta-per-city` | ETA from city + stock |
| B0484 | fulfillment | `eta-sla-metric` | Time-in-status histograms |
| B0485 | fulfillment | `failed-shipment-inventory` | FAILED restock vs scrap policy |
| B0486 | fulfillment | `handed-over-requires-carrier` | Require carrier on HANDED_OVER |
| B0487 | fulfillment | `label-path-objectstore` | Store labels in MinIO |
| B0488 | fulfillment | `label-zpl` | ZPL label support for warehouses |
| B0489 | fulfillment | `multi-package` | Multi-package shipment support |
| B0490 | fulfillment | `package-tests` | Tests for StartFulfillment transitions |
| B0491 | fulfillment | `packed-requires-tracking-optional` | Config: PACKED may omit tracking |
| B0492 | fulfillment | `packing-slip-pdf` | Generate packing slip PDF |
| B0493 | fulfillment | `photo-pod` | Photo proof of delivery |
| B0494 | fulfillment | `pickup-locker` | Pickup locker integration research |
| B0495 | fulfillment | `returns-label` | Return shipping label generation |
| B0496 | fulfillment | `route-optimization` | Delivery route optimization research |
| B0497 | fulfillment | `runner-failed-auto` | Auto FAILED after N days IN_TRANSIT |
| B0498 | fulfillment | `runner-feature-flag` | Gate auto fulfillment runner |
| B0499 | fulfillment | `shipment-events-outbox` | Emit shipment.* domain events |
| B0500 | fulfillment | `sla-by-city` | SLA targets by city |
| B0501 | fulfillment | `sms-eta` | SMS ETA updates |
| B0502 | fulfillment | `start-fulfillment-already-shipped` | Clear errors for bad StartFulfillment |
| B0503 | fulfillment | `tracking-url-template` | Carrier tracking URL templates |
| B0504 | fulfillment | `transition-idempotency-body` | Hash transition body into idem key |
| B0505 | infra | `arm64-images` | Build arm64 images |
| B0506 | infra | `backup-encryption` | Encrypt DB backups at rest |
| B0507 | infra | `caddy-openapi-docs-auth` | Protect /docs in prod |
| B0508 | infra | `canary-deploy` | Canary deploy strategy compose/k8s |
| B0509 | infra | `compose-dev-hot-reload` | air/reflex mounts in dev |
| B0510 | infra | `compose-dev-windows-paths` | Windows volume mount quirks |
| B0511 | infra | `compose-staging-overlay` | Staging overlay between dev/prod |
| B0512 | infra | `compose-staging-secrets` | Staging port publish parity |
| B0513 | infra | `demo-keys-gitignore-harden` | Harden demo key policy |
| B0514 | infra | `demo-readme-threat-model` | Never use demo keys in prod |
| B0515 | infra | `egress-proxy` | Forced egress proxy for Stripe |
| B0516 | infra | `feature-flag-remote` | Runtime feature flag source |
| B0517 | infra | `gpu-not-needed` | Document GPU N/A |
| B0518 | infra | `image-signing-cosign` | Sign images with cosign |
| B0519 | infra | `ipv6-ssh-publish` | Document IPv6 SSH publish |
| B0520 | infra | `kubernetes-helm` | Optional Helm chart |
| B0521 | infra | `migration-rollback-pairs` | Rollback notes 001-032 |
| B0522 | infra | `minio-cdn-front` | CDN in front of MinIO |
| B0523 | infra | `minio-image-variants` | webp+thumbnail on upload |
| B0524 | infra | `minio-init-version-pin` | Pin mc/minio digests |
| B0525 | infra | `minio-lifecycle-rules` | Lifecycle expire incomplete multipart |
| B0526 | infra | `minio-orphan-gc` | GC orphan product images |
| B0527 | infra | `multi-region-active-passive` | Active-passive multi-region design |
| B0528 | infra | `nats-jetstream-bootstrap` | Idempotent stream bootstrap |
| B0529 | infra | `network-segmentation` | Separate data vs edge networks |
| B0530 | infra | `postgres-16-extensions` | Document required extensions |
| B0531 | infra | `postgres-ha` | Postgres HA pattern doc |
| B0532 | infra | `postgres-resource-limits` | Compose mem limits for postgres |
| B0533 | infra | `prod-restart-budgets` | Ordered health-gated restart |
| B0534 | infra | `redis-acl-users` | Separate Redis ACLs |
| B0535 | infra | `redis-persistence-aof` | AOF persistence for Redis prod |
| B0536 | infra | `resource-limits-compose` | mem/cpu limits prod |
| B0537 | infra | `spot-instances-jobs` | Spot for batch jobs |
| B0538 | infra | `terraform-optional` | Optional Terraform for VPS |
| B0539 | infra | `wireguard-bastion` | WireGuard bastion for admin |
| B0540 | inventory | `abc-classification` | ABC inventory classification |
| B0541 | inventory | `admin-reservations-filter` | Filter by status/sku indexed |
| B0542 | inventory | `ats-available-to-sell` | ATS channel allocation |
| B0543 | inventory | `available-qty-formula-doc` | Document available = on_hand - reserved - safety |
| B0544 | inventory | `consignment-stock` | Consignment stock model |
| B0545 | inventory | `cycle-count-ui` | Admin cycle count workflow |
| B0546 | inventory | `expiry-date-beer` | Best-before tracking |
| B0547 | inventory | `incoming-stock-po` | Purchase order incoming stock |
| B0548 | inventory | `lot-batch-tracking` | Lot/batch tracking for recalls |
| B0549 | inventory | `negative-stock-page` | Pager on negative stock |
| B0550 | inventory | `n-plus-one-admin-reservations` | Batch SKU load for admin list |
| B0551 | inventory | `on-hand-check-constraint` | CHECK on_hand/reserved invariants |
| B0552 | inventory | `over-sell-race-bench` | Bench concurrent reserve low stock |
| B0553 | inventory | `package-unit-tests` | Unit tests for Reserve/Release/Commit |
| B0554 | inventory | `reconcile-job-tests` | Tests for reconcile Fix modes |
| B0555 | inventory | `reconcile-metric-labels` | Per-reason reconcile counters |
| B0556 | inventory | `reservation-product-idx` | Index reservations(product_id,status) |
| B0557 | inventory | `reservation-release-metrics` | Counters for release success/fail |
| B0558 | inventory | `reservation-ttl-config` | Configurable reservation expiry grace |
| B0559 | inventory | `reservation-unique-active` | Partial unique ACTIVE reservations |
| B0560 | inventory | `reserve-batch-all-or-nothing` | All line items reserve in one TX or none |
| B0561 | inventory | `reserve-serializable-flag` | Feature-flag serializable isolation |
| B0562 | inventory | `safety-stock-admin-api` | Admin PATCH safety_stock with audit |
| B0563 | inventory | `safety-stock-by-city` | Safety stock per ship-from |
| B0564 | inventory | `safety-stock-in-available` | available_qty subtracts safety_stock |
| B0565 | inventory | `sku-barcode` | Barcode/QR on SKU for warehouse |
| B0566 | inventory | `sku-level-hold` | Temporary SKU hold without delete |
| B0567 | inventory | `soft-hold-extend-api` | Extend reservation TTL via admin |
| B0568 | inventory | `transfer-between-warehouses` | Warehouse transfer orders |
| B0569 | inventory | `warehouse-multi-location` | Multi-warehouse inventory locations |
| B0570 | inventory | `weight-grams-null` | Reject unweighed SKUs at estimate |
| B0571 | observability | `alert-http-5xx` | Burn-rate 5xx alerts |
| B0572 | observability | `alert-inv-negative` | Page on negative inventory |
| B0573 | observability | `alert-orders-past-expires` | Alert reaper stuck |
| B0574 | observability | `alert-outbox-oldest` | Alert outbox lag SLO |
| B0575 | observability | `alert-rate-limited` | 429 ratio alerts |
| B0576 | observability | `business-kpis` | Business KPI metrics (GMV COP) |
| B0577 | observability | `caddy-access-metrics` | Caddy upstream latency |
| B0578 | observability | `cardinality-governor` | Drop high-cardinality labels |
| B0579 | observability | `continuous-profiling` | Continuous profiling Pyroscope/pprof |
| B0580 | observability | `custom-sli-definitions` | Formal SLI definitions doc |
| B0581 | observability | `db-pool-metrics` | sql.DB pool gauges |
| B0582 | observability | `dlq-age-gauge` | Oldest DLQ age gauge |
| B0583 | observability | `error-budget-policy` | Error budget policy doc |
| B0584 | observability | `exemplars-traces` | Prometheus exemplars linking traces |
| B0585 | observability | `grafana-dashboard-checkin` | Commit grafana JSON under docs/observability |
| B0586 | observability | `hmac-reject-total` | HMAC reject by reason |
| B0587 | observability | `job-run-metrics` | job runs_total last_success |
| B0588 | observability | `log-quality-score` | Lint structured log fields |
| B0589 | observability | `log-sampling-rules` | Per-route log sampling rules |
| B0590 | observability | `namespace-label` | Namespace label on all metrics |
| B0591 | observability | `otlp-exporter` | Optional OTLP export |
| B0592 | observability | `outbox-lag-slo` | SLO for outbox oldest pending |
| B0593 | observability | `prometheus-alerts-checkin` | Commit alerts.yml under docs/observability |
| B0594 | observability | `prom-recording-rules` | Recording rules checkout latency |
| B0595 | observability | `readyz-dependency-detail` | JSON body of dependency status |
| B0596 | observability | `refund-success-ratio` | Refunds status metrics |
| B0597 | observability | `rum-not-applicable` | Document RUM N/A for SSH |
| B0598 | observability | `saturation-metrics` | CPU/mem saturation for containers |
| B0599 | observability | `slo-burn-alerts` | Multi-window burn-rate alerts |
| B0600 | observability | `stripe-webhook-latency` | Histogram event to processed |
| B0601 | observability | `structured-audit-query` | LogQL examples for request_id |
| B0602 | observability | `subscription-mrr-gauge` | Active subscription gauges |
| B0603 | observability | `synthetic-ssh-probe` | Synthetic SSH login probe |
| B0604 | observability | `trace-context-propagation` | W3C traceparent end-to-end |
| B0605 | observability | `trace-sampling-rules` | Head/tail based sampling |
| B0606 | observability | `tui-client-errors` | Gateway API error by code |
| B0607 | observability | `tui-error-telemetry` | Anonymized TUI error counts |
| B0608 | ops | `backup-minio-bucket` | Include MinIO in backup drills |
| B0609 | ops | `backup-verify-checksum` | Checksum verify after restore |
| B0610 | ops | `blue-green-migrate` | Zero-downtime schema playbook |
| B0611 | ops | `caddy-mtls-admin` | Optional mTLS for /admin |
| B0612 | ops | `caddy-rate-limit` | Edge RL for webhooks and email |
| B0613 | ops | `capacity-planning` | Capacity planning spreadsheet |
| B0614 | ops | `chaos-kill-postgres` | Chaos drill kill postgres |
| B0615 | ops | `compose-prod-minio-tls` | Refuse default MinIO secrets |
| B0616 | ops | `compose-prod-no-stub-stripe` | Fail if STRIPE_MOCK=1 in prod |
| B0617 | ops | `compose-profiles-jobs` | Compose profiles for optional jobs |
| B0618 | ops | `config-drift-detect` | Detect compose vs running config drift |
| B0619 | ops | `core-api-behind-caddy-only` | Verify ports and internal DNS |
| B0620 | ops | `cost-dashboard` | Infra cost dashboard tags |
| B0621 | ops | `db-migration-window` | Document migration windows |
| B0622 | ops | `digest-pinning-ci` | Scheduled digest refresh PRs |
| B0623 | ops | `disaster-recovery-rto` | Define RTO/RPO numerically |
| B0624 | ops | `env-prod-example-parity` | CI diff env example vs Load* |
| B0625 | ops | `fulfillment-runner-once-docs` | Document fulfillment-runner flags |
| B0626 | ops | `inventory-reconcile-schedule` | Recommended reconcile cron |
| B0627 | ops | `inventory-snapshot-job` | Nightly inventory snapshot |
| B0628 | ops | `job-liveness-probes` | Healthchecks for all job services |
| B0629 | ops | `job-liveness-sidecar` | HEALTHCHECK for long-running jobs |
| B0630 | ops | `log-sampling-prod` | Sample high-cardinality logs |
| B0631 | ops | `nats-consumer-hpa` | Document HPA for consumers |
| B0632 | ops | `nats-js-backup` | JetStream offset backups |
| B0633 | ops | `pilot-stripe-webhook-smoke` | Wire stripe-webhook-smoke to deploy |
| B0634 | ops | `postgres-backup-verify` | Automated restore drill |
| B0635 | ops | `postgres-pit-restore-drill` | Quarterly PITR drills |
| B0636 | ops | `postmortem-template` | Incident postmortem template |
| B0637 | ops | `prod-secrets-checklist` | Validate .env.prod required keys |
| B0638 | ops | `redis-failover-runbook` | Redis outage impact runbook |
| B0639 | ops | `rotate-demo-ssh-keys` | Scripted demo keypair rotation |
| B0640 | ops | `runbook-auto-link` | Link alerts to runbooks |
| B0641 | ops | `secret-expiry-calendar` | Secret expiry calendar |
| B0642 | ops | `secret-manager-integration` | Vault/Docker secrets for prod |
| B0643 | ops | `secrets-rotation-runbook` | Automate secret rotation jobs |
| B0644 | ops | `staging-stripe-test-mode` | Document test-mode Stripe for staging |
| B0645 | ops | `status-page` | Public status page |
| B0646 | ops | `stripe-cli-forward-prod-ban` | Ban stripe listen to prod |
| B0647 | ops | `uptime-kuma-or-healthchecks` | External uptime for SSH and HTTPS |
| B0648 | ops | `war-game-schedule` | Quarterly war game schedule |
| B0649 | orders | `admin-status-override-guards` | Dual-control for PAID cancels |
| B0650 | orders | `backorder-policy` | Backorder accept/reject policy |
| B0651 | orders | `cancel-expired-observability` | Metrics for cancel-expired job |
| B0652 | orders | `cancel-paid-guardrails` | Reason codes + dual-control |
| B0653 | orders | `cancel-releases-reservations-metric` | Metric when cancel fails to release |
| B0654 | orders | `channel-attribution` | Channel attribution SSH vs API |
| B0655 | orders | `concurrent-convert-lock` | FOR UPDATE cart on convert |
| B0656 | orders | `customer-order-etag` | ETag on GET /orders/{id} for poll |
| B0657 | orders | `delivery-pii-column-encrypt` | Encrypt delivery PII at rest |
| B0658 | orders | `display-ref-collision` | Retry on unique display_ref |
| B0659 | orders | `display-ref-collision-tests` | Harden display_ref uniqueness |
| B0660 | orders | `draft-order-state-adr0021` | Implement OrderStatusDraft write path |
| B0661 | orders | `duplicate-order-detection` | Detect duplicate orders same cart hash |
| B0662 | orders | `exchange-rma` | RMA/exchange flow |
| B0663 | orders | `expire-grace-config` | Typed expire grace validation |
| B0664 | orders | `fraud-score-stub` | Fraud score stub feature flag |
| B0665 | orders | `gift-wrap-sku` | Gift wrap as add-on line |
| B0666 | orders | `items-sku-snapshot` | Snapshot SKU/name/price on items |
| B0667 | orders | `list-has-more-consistency` | limit+1 has_more pattern |
| B0668 | orders | `mark-paid-idempotency` | Test MarkPaid vs duplicate webhook |
| B0669 | orders | `markpaid-reaper-fence` | Metrics for reaper-vs-paid races |
| B0670 | orders | `multi-currency-block` | Reject non-COP at order create |
| B0671 | orders | `order-item-weight-snapshot` | Persist line weight at order time |
| B0672 | orders | `outbox-in-same-tx` | Audit MarkPaid/Cancel outbox TX |
| B0673 | orders | `pending-payment-cap` | Cap concurrent PENDING_PAYMENT |
| B0674 | orders | `pending-payment-reminder-email` | Email before expires_at |
| B0675 | orders | `preorder-status` | Preorder status before stock |
| B0676 | orders | `priority-shipping` | Priority shipping SKU |
| B0677 | orders | `quote-expiry` | Quote/DRAFT expiry job |
| B0678 | orders | `refunded-blocks-fulfillment` | REFUNDED cannot enter FULFILLING |
| B0679 | orders | `scheduled-order` | Schedule order for future date |
| B0680 | orders | `shipping-minor-snapshot` | Snapshot shipping quote on order row |
| B0681 | orders | `split-payment-future` | Research split payment feasibility |
| B0682 | orders | `subtotal-recalc-guard` | Reject checkout if client subtotal mismatches |
| B0683 | orders | `tax-engine-plug` | Pluggable tax engine interface |
| B0684 | orders | `uuid-vs-ulid-decision` | Decide Order ID strategy ADR |
| B0685 | outbox | `backoff-jitter` | Jitter OUTBOX_BASE_BACKOFF_SECONDS |
| B0686 | outbox | `backpressure-signal` | Signal producers when backlog high |
| B0687 | outbox | `cdc-alternative` | Evaluate CDC vs outbox long-term |
| B0688 | outbox | `consumer-processed-ttl` | Reap consumer_processed_messages |
| B0689 | outbox | `cross-region-replication` | Cross-region outbox replication |
| B0690 | outbox | `dedupe-key` | Producer-side dedupe key column |
| B0691 | outbox | `encryption-at-rest-payload` | Encrypt sensitive outbox payloads |
| B0692 | outbox | `exactly-once-docs` | Document at-least-once vs idempotent handlers |
| B0693 | outbox | `locked-by-hostname` | Default OUTBOX_LOCKED_BY to hostname |
| B0694 | outbox | `lock-ttl-config` | Configurable locked_at reclaim |
| B0695 | outbox | `max-attempts-per-type` | Per event_type max attempts override |
| B0696 | outbox | `multi-node-chaos-ci` | Always-on multi-node dispatch CI |
| B0697 | outbox | `nats-ack-timeout-metric` | Alert on ack failure spikes |
| B0698 | outbox | `nats-ack-timeout-tuning` | Document JetStream ack wait |
| B0699 | outbox | `nats-dlq-subject` | Dedicated NATS DLQ subject |
| B0700 | outbox | `ordering-per-aggregate` | Claim by aggregate sequence |
| B0701 | outbox | `payload-size-limit` | Reject oversized payload_json |
| B0702 | outbox | `poison-message-cap` | Move to DLQ after N attempts |
| B0703 | outbox | `poison-message-quarantine` | Admin replay for poison outbox |
| B0704 | outbox | `priority-lanes` | Priority lanes for order.paid |
| B0705 | outbox | `prometheus-exemplars` | Exemplars from outbox event ids |
| B0706 | outbox | `publisher-metrics` | Dispatch claimed/sent/failed counters |
| B0707 | outbox | `replay-authz-split` | Support list vs Ops replay |
| B0708 | outbox | `replay-from-backup` | Replay outbox from backup dump |
| B0709 | outbox | `replay-job-safety` | Dry-run + event id list for replay |
| B0710 | outbox | `schema-compat-ci` | CI check event payload compat |
| B0711 | outbox | `schema-evolution-compat` | Tests for envelope compat |
| B0712 | outbox | `schema-registry` | Optional schema registry for payloads |
| B0713 | outbox | `seed-integration-cleanup` | Guard outbox seed outside prod |
| B0714 | outbox | `seed-integration-guard` | FEATURE_FLAGS guard for seed helpers |
| B0715 | outbox | `subject-layout-doc-adr0016` | Check in ADR 0016 markdown |
| B0716 | outbox | `transactional-inbox` | Inbox pattern for external webhooks |
| B0717 | payments | `3ds-any-pilot-flag` | Feature-flag 3DS any for pilot |
| B0718 | payments | `3ds-return-ux` | Surface next_action URLs in TUI |
| B0719 | payments | `allowed-currencies-fail-prod` | Refuse wildcard currencies in prod |
| B0720 | payments | `billing-cycle-anchor` | Expose next_invoice_at on subscription |
| B0721 | payments | `card-on-file-mandate` | Mandate management for CO |
| B0722 | payments | `chargeback-playbook` | Chargeback response playbook |
| B0723 | payments | `checkout-locale-es` | Force Stripe Checkout locale es-419 |
| B0724 | payments | `checkout-payment-method-types` | Explicit payment_method_types for Colombia |
| B0725 | payments | `checkout-session-completed-race` | Serialize MarkPaid paths |
| B0726 | payments | `checkout-session-expiry-sync` | Cancel order on session expired |
| B0727 | payments | `cop-zero-decimal-guards` | COP never multiply by 100 |
| B0728 | payments | `currency-mismatch-guard` | Reject Stripe currency mismatches |
| B0729 | payments | `dead-letter-alert` | Alert high Stripe DLQ rate |
| B0730 | payments | `fx-fee-transparency` | Document Stripe FX fees N/A COP |
| B0731 | payments | `fx-not-supported` | Document FX conversion out of scope |
| B0732 | payments | `installments-research` | Installments feasibility Colombia |
| B0733 | payments | `local-payment-methods-co` | Evaluate PSE/Nequi via Stripe |
| B0734 | payments | `manual-capture-flag` | Manual capture feature flag |
| B0735 | payments | `manual-reconcile-audit` | Audit admin reconcile-stripe |
| B0736 | payments | `order-refunded-consumer` | NATS consumer for refunded events |
| B0737 | payments | `order-refunded-outbox-emit` | Emit orders.order.refunded outbox |
| B0738 | payments | `partial-refund-order-status` | PARTIALLY_REFUNDED status |
| B0739 | payments | `payment-status-history-api` | Expose payment status history |
| B0740 | payments | `payout-reconciliation` | Payout reconciliation report |
| B0741 | payments | `pm-detach-on-delete` | Detach Stripe PM on local delete |
| B0742 | payments | `proration-policy` | Document proration on qty updates |
| B0743 | payments | `radar-rules` | Stripe Radar rules for CO |
| B0744 | payments | `reconcile-subscription-gap` | Reconcile pending subscription checkouts |
| B0745 | payments | `refund-failed-status` | Propagate refund failed/canceled |
| B0746 | payments | `refund-idempotency-stripe-key` | Always pass Stripe Idempotency-Key |
| B0747 | payments | `refund-inventory-restock-policy` | Optional restock on refund |
| B0748 | payments | `refund-partial-cap` | Validate cumulative partial refunds |
| B0749 | payments | `refund-reason-enum` | Constrain refund reasons to allowlist |
| B0750 | payments | `revenue-recognition` | Revenue recognition reporting |
| B0751 | payments | `settlement-report` | Daily Stripe settlement report |
| B0752 | payments | `stripe-async-payment` | Async payment succeeded/failed |
| B0753 | payments | `stripe-customer-create-idempotent` | Idempotent Stripe Customer create by fingerprint |
| B0754 | payments | `stripe-dead-letter-alert` | Alert on Stripe DLQ growth |
| B0755 | payments | `stripe-dispute-webhook` | Handle charge.dispute.* freeze fulfill |
| B0756 | payments | `stripe-mock-parity-matrix` | Document mock vs live differences |
| B0757 | payments | `stripe-pi-failed-webhook` | Handle payment_intent.payment_failed |
| B0758 | payments | `stripe-reconcile-queue-metrics` | Gauges for reconcile queue depth |
| B0759 | payments | `stripe-tax-disabled-doc` | Document Stripe Tax off for COP MVP |
| B0760 | payments | `stripe-webhook-event-coverage` | Expand webhook event types handled |
| B0761 | payments | `subscription-cancel-at-period-end-ux` | Expose cancel_at_period_end |
| B0762 | payments | `subscription-invoice-paid` | invoice.paid/failed for renewals |
| B0763 | payments | `subscription-period-inventory` | Reserve inventory each billing cycle |
| B0764 | payments | `subscription-updated-webhook` | Sync subscription.updated/deleted |
| B0765 | payments | `terminal-shop-parity-matrix` | Matrix Terminal.shop vs Sip-Happens APIs |
| B0766 | payments | `webhook-event-log-ttl` | Retention job for stripe_event_log |
| B0767 | payments | `webhook-signature-rotation` | Dual webhook secrets during rotation |
| B0768 | performance | `admin-orders-n1` | Eliminate N+1 admin order list |
| B0769 | performance | `arena-alloc-research` | Go arena/alloc research hot paths |
| B0770 | performance | `batch-insert-order-items` | Batch insert order_items |
| B0771 | performance | `benchmark-ci` | Track bench regressions in CI |
| B0772 | performance | `cart-get-projection` | Slim cart DTO for TUI |
| B0773 | performance | `catalog-cache-invalidate` | Pub/sub invalidate on admin price |
| B0774 | performance | `catalog-cache-singleflight` | Singleflight catalog SWR |
| B0775 | performance | `catalog-search-trgm` | pg_trgm for catalog search |
| B0776 | performance | `cold-start-jobs` | Reduce job binary cold start |
| B0777 | performance | `connection-pool-tuning` | Document DBPool vs load |
| B0778 | performance | `conn-pool-per-job` | Separate DB pools per job type |
| B0779 | performance | `gzip-json-responses` | Optional gzip for large JSON |
| B0780 | performance | `hmac-body-buffer-limit` | Cap HMAC body ReadAll size |
| B0781 | performance | `http2-cleartext-h2c` | Evaluate h2c between gateway and api |
| B0782 | performance | `http-timeouts-server` | Explicit Read/Write/Idle timeouts |
| B0783 | performance | `idempotency-store-index` | Index idempotency keys hot path |
| B0784 | performance | `json-encoder-pool` | Pool JSON encoders |
| B0785 | performance | `lazy-metrics-register` | Lazy register expensive collectors |
| B0786 | performance | `mmap-catalog-research` | Research mmap for catalog cache |
| B0787 | performance | `nats-consume-parallel-handlers` | Worker pool for order.paid |
| B0788 | performance | `nonce-store-index` | hmac_nonces expiry index |
| B0789 | performance | `orders-list-index` | Verify orders customer indexes |
| B0790 | performance | `outbox-batch-adaptive` | Adaptive outbox batch size |
| B0791 | performance | `pgo-build` | Optional PGO builds for api |
| B0792 | performance | `postgres-stat-statements` | Enable pg_stat_statements in dev |
| B0793 | performance | `prepared-statements` | Use prepared statements for hot paths |
| B0794 | performance | `query-plan-ci` | Explain plan checks for hot SQL |
| B0795 | performance | `redis-pipeline-ratelimit` | Pipeline Redis RL ops |
| B0796 | performance | `redis-rl-fail-closed` | Fail closed Redis RL in prod |
| B0797 | performance | `tui-catalog-virtualization` | Virtualize long catalogs |
| B0798 | performance | `view-init-parallelism` | Timeouts per view/init section |
| B0799 | performance | `zero-alloc-hotpath` | Zero-alloc goals for HMAC verify |
| B0800 | product | `ab-test-framework` | A/B test framework for TUI copy |
| B0801 | product | `abv-warning-strong` | Extra warning for high ABV SKUs |
| B0802 | product | `admin-partial-capture` | Auth-only Stripe flows |
| B0803 | product | `age-gate-document-upload` | Stronger age verification |
| B0804 | product | `age-gate-remember-device` | Short-lived age affirmation |
| B0805 | product | `beer-club-tier` | Beer club subscription tiers |
| B0806 | product | `brewery-pages` | Browse by brewery in TUI |
| B0807 | product | `brewery-portal` | Brewery self-serve portal research |
| B0808 | product | `bundle-skus` | Product bundles as cart lines |
| B0809 | product | `cart-merge-on-login` | Merge policy fingerprint vs OAuth cart |
| B0810 | product | `collab-cart` | Collaborative cart research |
| B0811 | product | `corporate-invoicing` | NIT billing for B2B |
| B0812 | product | `customer-tags` | Admin tags on customers |
| B0813 | product | `draft-quotes-b2b` | B2B quotes via DRAFT orders |
| B0814 | product | `feature-flag-newsletter` | Gate public /email |
| B0815 | product | `feature-flag-oauth` | Gate oauth_apps_enabled |
| B0816 | product | `feature-flag-status-override` | Ops UI warning when enabled |
| B0817 | product | `feature-flag-subscriptions` | Gate subscriptions_enabled |
| B0818 | product | `gift-cards` | Gift cards product research |
| B0819 | product | `gift-message-checkout` | Gift note on checkout |
| B0820 | product | `invoice-email-copy` | Email copy of electronic invoice |
| B0821 | product | `limited-drop` | Limited drop / queue purchase |
| B0822 | product | `loyalty-points` | Loyalty points stub behind flag |
| B0823 | product | `marketplace-multi-vendor` | Multi-vendor marketplace research |
| B0824 | product | `more-cities-shipping` | More Colombian cities |
| B0825 | product | `multi-item-weight-shipping` | Sum weight for tiers |
| B0826 | product | `newsletter-from-tui` | TUI opt-in POST /email |
| B0827 | product | `nfc-tap-future` | NFC tap marketing research |
| B0828 | product | `nft-not-planned` | Document NFT out of scope |
| B0829 | product | `offline-catalog-cache-tui` | Degraded catalog cache |
| B0830 | product | `order-notes-internal` | Internal ops notes on orders |
| B0831 | product | `partial-fulfillment` | Split shipments / partial cancel |
| B0832 | product | `pat-scripts-gallery` | Example Bearer PAT scripts |
| B0833 | product | `pickup-point` | Pickup point vs delivery option |
| B0834 | product | `promo-codes` | Promo code engine feature-flagged |
| B0835 | product | `reorder-from-history` | One-key reorder from history |
| B0836 | product | `seasonal-featured` | Seasonal featured collections |
| B0837 | product | `ssh-only-marketing-site` | Public page explain SSH shop |
| B0838 | product | `stock-waitlist` | Back-in-stock email notify |
| B0839 | product | `style-taxonomy` | Canonical beer style taxonomy |
| B0840 | product | `subscription-skip-cycle` | Skip billing cycle Stripe pause |
| B0841 | product | `subscription-skip-delivery` | Skip next delivery cycle |
| B0842 | product | `taproom-events` | Taproom event tickets research |
| B0843 | product | `tastings-rsvp` | Tasting event RSVP |
| B0844 | product | `wishlist-flag` | Wishlist only behind flag+holds |
| B0845 | refactor | `admin-mux-uniform` | Uniform admin middleware stack |
| B0846 | refactor | `cmd-jobs-subcommand-registry` | Registry pattern for job commands |
| B0847 | refactor | `config-validation-struct` | Validate config with go-playground |
| B0848 | refactor | `context-timeouts-audit` | Audit missing context timeouts |
| B0849 | refactor | `customer-id-from-request` | Single identity helper |
| B0850 | refactor | `customers-service-iface` | Clarify customers.Service mocks |
| B0851 | refactor | `delete-dead-code-scan` | Periodic deadcode scan |
| B0852 | refactor | `domain-event-builders` | Builders for all TypeOrders* |
| B0853 | refactor | `domain-events-versioning` | Explicit event schema versions |
| B0854 | refactor | `empty-pkg-observability` | Implement or remove empty pkg |
| B0855 | refactor | `error-code-registry` | Shared error code registry |
| B0856 | refactor | `errors-is-as-audit` | Audit fmt.Errorf wrapping with %w |
| B0857 | refactor | `event-sourcing-eval` | Evaluate event sourcing for orders |
| B0858 | refactor | `extract-checkout-engine` | Extract checkout engine package |
| B0859 | refactor | `extract-shipping-service` | Extract shipping as service |
| B0860 | refactor | `featureflags-known-complete` | Known for all risky flags |
| B0861 | refactor | `functional-options-config` | Functional options for Server |
| B0862 | refactor | `handlefunc-to-handle` | Method-aware admin routes |
| B0863 | refactor | `hexagonal-ports` | Hexagonal ports for Stripe/NATS |
| B0864 | refactor | `http-handler-packages` | Split http into domain packages |
| B0865 | refactor | `invoicing-unknown-provider-fatal` | Unknown provider hard error |
| B0866 | refactor | `module-split-future` | Criteria to split go module |
| B0867 | refactor | `pagination-helpers` | Unify parseLimit/offset/has_more |
| B0868 | refactor | `reduce-global-state-tui` | Eliminate global mutable TUI state |
| B0869 | refactor | `reduce-interface-bloat` | Trim unnecessary interfaces |
| B0870 | refactor | `remove-legacy-hmac-path` | Delete HMAC v1 after migration |
| B0871 | refactor | `server-go-god-object` | Break server.go wiring |
| B0872 | refactor | `shared-pagination-type` | Shared pagination response type |
| B0873 | refactor | `stripe-event-switch` | Typed webhook handler registry |
| B0874 | refactor | `tui-api-dto-codegen` | Generate DTOs from OpenAPI |
| B0875 | security | ` pentest-scope` | Define pentest scope for SSH+API |
| B0876 | security | `access-log-body-ban` | Never log PII bodies |
| B0877 | security | `admin-console-csrf` | CSRF for future cookie auth |
| B0878 | security | `admin-console-xss` | CSP escape admin console |
| B0879 | security | `admin-export-column-filter` | Default exclude PII columns |
| B0880 | security | `admin-export-role` | Require finance on export |
| B0881 | security | `attack-surface-diagram` | Attack surface diagram |
| B0882 | security | `bug-bounty-scope` | Bug bounty program draft |
| B0883 | security | `content-type-sniff-admin` | nosniff on admin HTML |
| B0884 | security | `csp-report-uri` | CSP report-uri for admin |
| B0885 | security | `demo-keys-prod-guard` | Refuse demo keys in prod |
| B0886 | security | `dependency-osv` | Fail CI on high OSV |
| B0887 | security | `email-confirm-token-binding` | Bind confirm tokens to host |
| B0888 | security | `escape-hatches-doc` | Document security escape hatches |
| B0889 | security | `govulncheck-block-transitive` | Remediate advisory findings |
| B0890 | security | `headers-security-baseline` | Security headers middleware |
| B0891 | security | `hmac-log-redaction` | Redact sig headers in debug |
| B0892 | security | `hsts-prod` | HSTS header on prod Caddy |
| B0893 | security | `image-upload-exif-strip` | Strip EXIF on upload |
| B0894 | security | `key-pinning-ssh` | SSH key pinning for ops hosts |
| B0895 | security | `least-privilege-iam` | Cloud IAM least privilege templates |
| B0896 | security | `minio-bucket-policy` | Private buckets + signed URLs |
| B0897 | security | `minio-public-acl` | Private bucket + CDN only images |
| B0898 | security | `namespace-prod-stripe-mock-forbidden` | Refuse prod+mock |
| B0899 | security | `oauth-secret-hash` | Hash oauth client secrets |
| B0900 | security | `password-n-a` | Document passwords N/A SSH keys only |
| B0901 | security | `pat-hash-at-rest` | Test list never returns plaintext |
| B0902 | security | `rate-limit-fingerprint` | RL keys include FP+route |
| B0903 | security | `rate-limit-headers` | RateLimit-Remaining headers |
| B0904 | security | `redact-pii-keys` | Redact email/phone/legal_id/address |
| B0905 | security | `sbom-attest` | Attest SBOM to images |
| B0906 | security | `secret-scanning-gitleaks` | gitleaks CI step |
| B0907 | security | `secrets-in-memory-scrub` | Scrub secrets from heap dumps |
| B0908 | security | `sql-injection-fuzz` | Fuzz catalog q and admin filters |
| B0909 | security | `ssrf-invoice-http` | SSRF allowlist for invoice HTTP |
| B0910 | security | `ssrf-success-url` | Block link-local in callbacks |
| B0911 | security | `stripe-webhook-ip-allow` | Optional Stripe IP allowlist |
| B0912 | security | `threat-intel-feed` | Optional IP threat intel feed |
| B0913 | security | `tls-min-version-caddy` | Enforce TLS1.2+ in Caddyfile |
| B0914 | security | `waf-rules` | WAF rules for HTTPS edge |
| B0915 | security | `webhook-ip-allowlist` | Stripe source IP allowlist flag |
| B0916 | ssh | `api-client-retry-jitter` | Backoff on api.Client 5xx/429 |
| B0917 | ssh | `auditd-integration` | Host auditd for SSH port |
| B0918 | ssh | `audit-log-session` | Session start/end audit log |
| B0919 | ssh | `authorized-keys-hot-reload` | Reload authorized_keys without restart |
| B0920 | ssh | `authorized-keys-reload` | Hot-reload keys without restart |
| B0921 | ssh | `banner-dynamic` | Dynamic banner with status |
| B0922 | ssh | `banner-legal-text` | SSH banner Ley 1581/alcohol notice |
| B0923 | ssh | `bearer-client-mode` | CLI mode with BearerRoundTripper |
| B0924 | ssh | `certificate-auth` | SSH certificate authority research |
| B0925 | ssh | `colorprofile-fallback` | Monochrome fallback on detect fail |
| B0926 | ssh | `connection-rate-syn` | SYN flood mitigations at host |
| B0927 | ssh | `disable-tcp-forward-test` | Test port forward denial |
| B0928 | ssh | `ed25519-only` | Reject non-ed25519 client keys in prod |
| B0929 | ssh | `fail2ban-integration` | fail2ban for SSH gateway port |
| B0930 | ssh | `fingerprint-deny-list` | Deny list of fingerprints |
| B0931 | ssh | `force-command-reject` | Reject ForceCommand attempts |
| B0932 | ssh | `gateway-metrics-alerts` | Alert connection refused vs auth fail |
| B0933 | ssh | `geo-block-optional` | Optional geo allowlist for SSH |
| B0934 | ssh | `host-key-rotation-automation` | Automate dual host-key roll |
| B0935 | ssh | `host-key-rotation-docs` | known_hosts rotation playbook |
| B0936 | ssh | `idle-timeout` | Configurable idle disconnect |
| B0937 | ssh | `idle-timeout-notice` | Warn before idle disconnect |
| B0938 | ssh | `keepalive-config` | Client/server keepalive tuning |
| B0939 | ssh | `login-grace-time` | Configure SSH login grace time |
| B0940 | ssh | `max-sessions-per-fp` | Concurrent session caps |
| B0941 | ssh | `max-sessions-per-key` | Limit concurrent sessions per FP |
| B0942 | ssh | `metrics-addr-default` | Default SSH_METRICS_ADDR in compose |
| B0943 | ssh | `mqtt-bridge-not-planned` | MQTT bridge out of scope |
| B0944 | ssh | `observability-session-metrics` | Session duration and auth fail metrics |
| B0945 | ssh | `ratelimit-per-user` | Align gateway RL with API FP keys |
| B0946 | ssh | `session-package-implement` | Implement empty session package |
| B0947 | ssh | `sftp-disable` | Explicitly disable SFTP subsystem |
| B0948 | ssh | `wish-middleware-recover` | Panic recover per SSH session |
| B0949 | testing | `admin-rl-bypass-regression` | adminRL regression test |
| B0950 | testing | `audit-package-tests` | Cover audit writers |
| B0951 | testing | `cart-concurrent-convert` | Two converts one order |
| B0952 | testing | `concurrent-checkout-chaos` | cart_convert vs reaper chaos |
| B0953 | testing | `contract-breaking-change` | Detect OpenAPI breaking changes |
| B0954 | testing | `contract-consumer-driven` | Consumer-driven contracts gateway |
| B0955 | testing | `contract-test-generator` | Generate contracts from OpenAPI |
| B0956 | testing | `e2e-playwright-admin` | Playwright for admin console |
| B0957 | testing | `e2e-smoke-subscriptions` | E2E smoke subscription create cancel |
| B0958 | testing | `emailsubscribe-reap-job` | Reap retention integration |
| B0959 | testing | `feature-flag-matrix-test` | Unknown flags warned at start |
| B0960 | testing | `flaky-detector` | Detect flaky tests from CI history |
| B0961 | testing | `fulfillment-zero-tests` | Table tests shipment transitions |
| B0962 | testing | `fuzz-display-ref` | Fuzz display_ref parser |
| B0963 | testing | `golden-hmac` | Golden vectors for HMAC v2 |
| B0964 | testing | `hmac-skew-boundary` | Property test ±5m skew |
| B0965 | testing | `inventory-reconcile-ci` | CI reconcile with leak fixture |
| B0966 | testing | `jobs-main-flag-parse` | Jobs subcommand usage tests |
| B0967 | testing | `load-test-k6` | k6 scripts for checkout load |
| B0968 | testing | `migrate-package-tests` | Migrate ordering and checksums |
| B0969 | testing | `migration-linearity-windows` | Linearity under Git Bash |
| B0970 | testing | `minio-upload-content-type` | Fuzz DetectImageContentType |
| B0971 | testing | `mutation-money` | Mutation test orders/payments |
| B0972 | testing | `oauth-confidential-client-basic` | HTTP Basic client auth test |
| B0973 | testing | `objectstore-minio-tests` | MinIO testcontainers tests |
| B0974 | testing | `openapi-drift-bodies` | Fail missing schema $refs |
| B0975 | testing | `openapi-response-examples` | Golden JSON fixtures |
| B0976 | testing | `outbox-skip-locked-multi` | No double-publish chaos |
| B0977 | testing | `pkg-auth-hmac-tests` | Unit tests pkg/auth |
| B0978 | testing | `pkg-featureflags-tests` | Parse/Has/All flag tests |
| B0979 | testing | `pkg-money-tests` | Unit tests pkg/money |
| B0980 | testing | `pkg-natshealth-tests` | natshealth.Ping tests |
| B0981 | testing | `pkg-redisutil-tests` | Redis util parse failures |
| B0982 | testing | `pkg-sshfp-tests` | sshfp fingerprint format tests |
| B0983 | testing | `property-based-money` | Property tests for money math |
| B0984 | testing | `redact-pii-unit` | PII never survives redact.JSON |
| B0985 | testing | `refund-charge-refunded-shapes` | Table-driven refund shapes |
| B0986 | testing | `refunds-webhook-e2e` | charge.refunded e2e |
| B0987 | testing | `resilience-chaos-mesh` | Chaos Mesh experiments k8s |
| B0988 | testing | `seed-demo-fingerprint-match` | Assert seed FP matches compose default |
| B0989 | testing | `snapshot-sql-schema` | Snapshot schema after migrations |
| B0990 | testing | `ssh-gateway-cmd-smoke` | Build/run gateway smoke |
| B0991 | testing | `ssh-ratelimit-integration` | Session fingerprint flood test |
| B0992 | testing | `tui-model-keys-exhaustive` | Every screen key map |
| B0993 | testing | `tui-resize-i18n` | es_CO reflow tests |
| B0994 | testing | `visual-regression-tui` | Visual regression for TUI frames |
| B0995 | testing | `webhook-subscription-e2e` | invoice.paid renewal e2e |
| B0996 | testing | `windows-path-migrate` | migrate on windows-latest |
| B0997 | testing | `workflows-saga-tests` | Test Saga.OnOrderPaid |
| B0998 | tui | `a11y-high-contrast` | High-contrast theme toggle |
| B0999 | tui | `a11y-reduce-motion` | No-blink mode for animations |
| B1000 | tui | `about-build-version` | Show API/gateway build version in about |
| B1001 | tui | `account-profile-edit` | Edit profile fields from account |
| B1002 | tui | `address-book-crud-ui` | Full address CRUD screens |
| B1003 | tui | `age-gate-dob-validation-ux` | Clearer DOB inline errors |
| B1004 | tui | `bookmarks` | Bookmark favorite SKUs |
| B1005 | tui | `braille-spinner` | Braille spinner for busy states |
| B1006 | tui | `busy-spinner-cancel` | Esc cancels in-flight API |
| B1007 | tui | `cart-line-remove-confirm` | Confirm before removing last line |
| B1008 | tui | `catalog-filter-persist` | Persist filters across sessions |
| B1009 | tui | `catalog-sort-options` | Sort by price/name/abv in filters |
| B1010 | tui | `checkout-city-picker-only` | City picker limited to canonical cities |
| B1011 | tui | `command-palette` | Command palette Ctrl-K style |
| B1012 | tui | `consumer-rights-deep-link` | OSC-8 hyperlinks for legal URLs |
| B1013 | tui | `consumer-rights-offline` | Cache legal footer URLs |
| B1014 | tui | `copy-mode-payment-url` | Keybind copy payment URL to clipboard OSC |
| B1015 | tui | `demo-mode-banner` | Banner when STRIPE_MOCK=1 |
| B1016 | tui | `empty-history-cta` | CTA from empty history to catalog |
| B1017 | tui | `english-locale` | Add messages_en selectable locale |
| B1018 | tui | `error-screen-retry` | Retry last failed Cmd |
| B1019 | tui | `faq-from-markdown` | Load FAQ from docs content |
| B1020 | tui | `figlet-optional` | Optional figlet splash |
| B1021 | tui | `filters-abv-validation` | Localize ABV parse errors |
| B1022 | tui | `form-field-errors` | Per-field checkout validation |
| B1023 | tui | `friendly-err-i18n` | Map all API codes to es_CO |
| B1024 | tui | `help-screen-keybind-map` | Generate help from keybind source |
| B1025 | tui | `help-subscriptions-tokens` | Help for sub and PAT keys |
| B1026 | tui | `messages-locale-switch` | Messages interface swap es/en |
| B1027 | tui | `mouse-support-optional` | Optional mouse wheel for catalog |
| B1028 | tui | `multi-language-runtime` | Runtime language switch without restart |
| B1029 | tui | `narrow-terminal-regression` | Expand resize tests width less than 60 |
| B1030 | tui | `oauth-client-management-ui` | Register/revoke OAuth clients in TUI |
| B1031 | tui | `offline-banner` | Persistent offline banner |
| B1032 | tui | `order-history-pagination` | Paginate order history |
| B1033 | tui | `partial-view-init` | Render catalog if orders fail |
| B1034 | tui | `payment-expired-copy` | Dedicated expired payment copy |
| B1035 | tui | `payment-method-list-ui` | List/select saved payment methods |
| B1036 | tui | `plugin-themes` | User-selectable color themes |
| B1037 | tui | `product-image-sixel` | Optional sixel/kitty images |
| B1038 | tui | `qr-fallback-copy` | URL-only fallback when QR fails |
| B1039 | tui | `qr-fallback-width` | Degrade QR on narrow width |
| B1040 | tui | `rate-limited-copy` | es_CO rate_limited guidance |
| B1041 | tui | `retry-backoff-cmd` | Central tea.Cmd retry with jitter |
| B1042 | tui | `screen-menu-dead-code` | Remove or wire screenMenu |
| B1043 | tui | `screen-reader-labels` | Stable ESTADO/TOTAL markers |
| B1044 | tui | `search-fuzzy` | Fuzzy catalog search |
| B1045 | tui | `splash-skip-key` | Allow key to skip splash |
| B1046 | tui | `split-model-go` | Split model.go into screen packages |
| B1047 | tui | `subscription-management-full` | Complete cancel/pause/edit UX |
| B1048 | tui | `token-scope-picker` | Scope checkboxes on PAT create |
| B1049 | tui | `token-show-once` | Wipe PAT plaintext after navigate |
| B1050 | tui | `tui-resize-i18n` | Assert es_CO reflow on resize |
| B1051 | tui | `tutorial-first-run` | First-run tutorial overlay |
| B1052 | tui | `unicode-width-audit` | Audit unicode width in lipgloss |
| B1053 | tui | `view-init-warning-ux` | Surface ViewInitWarning in banner |
| B1054 | tui | `vim-keys-optional` | Optional hjkl navigation |
| B1055 | tui | `zen-mode` | Zen mode hide chrome |

## Roadmap ID index (legacy comments)

| ID | Topic |
|----|--------|
| 040 | Admin RBAC tiers |
| 138 | Shipping status emails |
| 139 | Compliance footer |
| 140 | Weight-tier shipping |
| 143 | Expired pending-order metrics |
| 150 | Pilot / post-deploy Stripe smoke |

## Definition of SSH-available

1. `ssh-gateway` on a host-reachable TCP port
2. `SSH_REQUIRE_AUTHORIZED_KEYS=1` with customer keys loaded
3. HMAC secrets set between gateway and API
4. Age gate → catalog → cart → Stripe URL end-to-end
