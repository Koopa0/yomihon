# Engineering exceptions

This directory holds the only valid departures from an applicable MUST or
MUST NOT in `ENGINEERING_STANDARD.md` or `PROJECT_PROFILE.md`.

Records are named `EX-YYYY-NNN-short-name.md` and use
`QUALITY_EXCEPTION.template.md`. An exception has no force until its status is
`Approved` and it names an independent approver, concrete risk, containment,
owner, objective expiry or trigger, closure condition, and maximum permitted
verdict. Passing the trigger changes the exception to FAIL until it is closed
or independently renewed.

Schedule pressure, a desired green check, and generic “follow up later” text
are not valid reasons. Exceptions cannot waive the product's privacy walls or
a known critical authority boundary.
