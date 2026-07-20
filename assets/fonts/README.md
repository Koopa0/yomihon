# Embedded fonts

The application embeds these self-hosted fonts so reading never causes a
third-party network request. They are licensed under the SIL Open Font
License 1.1 in [LICENSE.txt](LICENSE.txt).

| Files | Upstream identity | SHA-256 |
|---|---|---|
| `Geist-Variable.woff2` | Geist 1.500, Geist Project Authors | `c46b00cf667277d22cc237e58149520daec19542edc3f05e7daff4581dc23d2a` |
| `GeistMono-Variable.woff2` | Geist Mono 1.500, Geist Project Authors | `78b4deef94de1cc4b63ba58ba86fe9e64b7f41aa8c6a7e2eb534e281834e94dd` |
| `Newsreader-Variable.woff2` | Newsreader 1.003, Newsreader Project Authors | `ac6fa9ed533278f4c8fd3ae44a1fc78c7df736040237ab86fc1160d020af0af2` |
| `Newsreader-Italic-Variable.woff2` | Newsreader Italic 1.003, Newsreader Project Authors | `d8c263970d52e0b94b3d5d4250d5962fe39f8f3b6fa9ad13b406d73ff3f4b036` |

Upstream projects:

- Geist: <https://github.com/vercel/geist-font>, release 1.5.0.
- Newsreader: <https://github.com/productiontype/Newsreader>.

`SHA256SUMS` is the machine-checked inventory for these four redistributed
files; `make license-check` verifies it before a merge or release claim.

The exact download URLs used for the four existing binaries were not retained.
Their embedded name, version, copyright, and licence metadata were verified
directly from the WOFF2 files on 2026-07-14; the hashes above identify the
redistributed bytes without claiming a source archive that cannot be proved.
