# Compute placement policy

`internal/compute.Place` is a pure, deterministic selection function for a future job provider. It does not create a job, reserve capacity, or call a worker. No placement API or job mutation endpoint is exposed by the current read-only demo.

Callers supply worker state, freshness, available and total CPU/RAM/GPU capacity, capabilities, tags, architecture, and optional load. A request may require resources and capabilities, restrict placement to specified workers, exclude workers, and preserve minimum free CPU/RAM after placement.

The policy rejects offline, drained, stale, incompatible, or undersized candidates and records a reason for each rejection. Eligible workers are scored by the mean fraction of capacity left after placement, plus GPU fraction when GPU is requested and inverse load when reported. The highest score wins; worker ID is the stable tie-break. The decision includes every candidate assessment so a future UI can explain it.

Before dispatch, a provider must recheck resources and persist the decision with the job. The current demo worker is stale and therefore ineligible. This package is groundwork for live compute integration, not evidence that jobs can run yet.
