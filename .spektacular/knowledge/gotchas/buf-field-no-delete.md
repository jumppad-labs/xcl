---
tags: [proto, buf, compatibility, coreservice]
---

# buf FILE rules refuse removing a reserved field

Under `breaking: use: [FILE]`, buf's `FIELD_NO_DELETE` refuses removing a field even when both its number and its name are reserved (checked with buf 1.27.2). To remove a field safely, except `FIELD_NO_DELETE` and add `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED`. With that set, deleting a field without reserving it still fails the check, and deleting it with `reserved <n>;` and `reserved "<name>";` passes.

`proto/buf.yaml` uses exactly this set since the `highlight` field of the two encode requests was retired, so a future field removal in `core.proto` must reserve both the number and the name: dropping either one fails `make buf-breaking` with "deleted without reserving the name/number".
