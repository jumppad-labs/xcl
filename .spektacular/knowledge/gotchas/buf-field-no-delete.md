---
tags: [proto, buf, compatibility, coreservice]
---

# buf FILE rules refuse removing a reserved field

Under `breaking: use: [FILE]`, buf's `FIELD_NO_DELETE` refuses removing a field even when both its number and its name are reserved (checked with buf 1.27.2). To remove a field safely, except `FIELD_NO_DELETE` and add `FIELD_NO_DELETE_UNLESS_NUMBER_RESERVED` and `FIELD_NO_DELETE_UNLESS_NAME_RESERVED`. With that set, deleting a field without reserving it still fails the check, and deleting it with `reserved <n>;` and `reserved "<name>";` passes.
