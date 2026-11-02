# Local build inspection evidence

`build-cache.json` is still the authoritative successful-build record. Its schema,
content fingerprint algorithm, artifact checks and build-skip semantics are
unchanged. `FingerprintContext` computes the same fingerprint as `Fingerprint`,
with cancellation checks during traversal and content reads.

`inspection-evidence.json` is separate, disposable, versioned machine-local
inspection state. Pass the same user-local workspace state directory used for
the build cache to `LoadEvidence` and `InspectionEvidence.Save`. Do not store it
in a portable workspace. Writes use the existing atomic JSON writer and mode
0600. Unsupported versions and malformed files return errors; callers can treat
these as unavailable evidence rather than preventing a build.

## Integration

1. Bypass **all** cache/evidence operations for `--no-cache`.
2. Capture `before := MetadataDigest(ctx, appPath, effectiveConfig, output, tool)`.
3. For fast status, `evidence.Lookup(stableAppKey, before)` returns a previously
   authoritative fingerprint if the metadata matches. Still check `Cache.Current`
   for that fingerprint and the expected artifact paths. A miss is not proof of
   a changed build: perform authoritative inspection or report pending/unknown.
4. For an authoritative inspection or successful build, run `FingerprintContext`,
   capture metadata again, and call `evidence.Record(key, before, after, fp)`.
   Save evidence only after successful inspection/build; do not record failed
   or canceled operations. `Record` invalidates old evidence if snapshots differ.
   It can record an inspection fingerprint that differs from the successful
   build cache, without modifying the successful-build record.
5. Serialize evidence mutations and saves, just as for `Cache`. Recheck the
   caller's context/request identity before publishing asynchronous results.

Metadata includes effective configuration, absolute application/output/utility
paths, utility size/mtime/mode, and ordered source paths/size/mtime/mode. Source
and utility contents are not read. Output and `.git` subtrees are excluded as in
the authoritative scan. Source links, non-regular files and unsafe paths are
rejected. Relocation invalidates this local evidence but not stable identities.

Metadata is **not proof of content equality**: an edit preserving size and mtime
can match it. Therefore it is only a fast status hint. Builds must always compute
the authoritative content fingerprint before deciding whether to skip packaging.
Even before/after metadata snapshots cannot detect every concurrent content edit.
A canceled metadata scan returns no partial snapshot. Persistence checks context
before the atomic write; cancellation during that write may still allow it to
complete.

`BenchmarkInspection` compares metadata and authoritative scans for small and
large file sets, reporting source-file counts and content-byte counts per
operation (metadata reads zero content bytes).
