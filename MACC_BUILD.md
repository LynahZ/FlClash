# MACC release builds

MACC server addresses require decoding material. Release builds embed the value
from the GitHub Actions repository secret `MACC_SALT`; published binaries expose
that value to anyone who can inspect them. Configure the secret before running
the MACC release workflow. Never commit the material to the repository.

The material must be supplied as the original UTF-8 text, without trailing line
breaks. For example, with GitHub CLI and a privately held file:

```sh
gh secret set MACC_SALT --repo OWNER/FlClash < /path/to/macc.salt
```

The placeholder `core/Clash.Meta/adapter/outbound/macc-default.salt` is empty in
source control. The packaging job replaces it in its temporary checkout and
verifies the embedded value before packaging. Builds made without that step
continue to require an explicit salt.

At runtime `MIHOMO_MACC_SALT` takes precedence, followed by `macc.salt` in the
core home and then the embedded release value. An override lets users provide
different material if the subscription provider changes formats or keys.
