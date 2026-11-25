# Third-party notices

The sources below are research references for the W000 documentation. The W000 source survey links to the upstream files and describes their behavior in original prose. This notice records the provenance of those references.

| Source | Pinned upstream license | Notices and attribution to review before reuse |
| --- | --- | --- |
| EF Core v10.0.15 | [MIT, LICENSE.txt](https://github.com/dotnet/efcore/blob/b34c0e9d7e768aa808ded44f19e7f871bea6869f/LICENSE.txt), .NET Foundation and Contributors | The pinned tree has no conventional separate notice filename. Check the source header and retain the upstream copyright and license text where required. |
| .NET runtime v10.0.0 | [MIT, LICENSE.TXT](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/LICENSE.TXT), .NET Foundation and Contributors | [THIRD-PARTY-NOTICES.TXT](https://github.com/dotnet/runtime/blob/60629d14374c56f1cb51819049ad1fa529307f8d/THIRD-PARTY-NOTICES.TXT) identifies components under separate terms. Review the specific file and relevant notices. |
| go-linq v5.0.0 and later research snapshot | Apache-2.0 at [stable LICENSE](https://github.com/ahmetb/go-linq/blob/6ebc43b7c971a8a4cb9e38742f6cb491d4032cbb/LICENSE) and [research LICENSE](https://github.com/ahmetb/go-linq/blob/91b0fe50132dbf3efddb832376e3fe3d8daa06d9/LICENSE) | Both trees contain the same license blob and no conventional separate notice filename. Section 4 covers redistribution, notices and attribution, including marking modified files and retaining relevant notices. Check source headers and any embedded attribution. |
| Npgsql EF Core provider v10.0.4 | [PostgreSQL License](https://github.com/npgsql/efcore.pg/blob/fe1109ecf52646ed6fc718a9a233353ef2e7baf9/LICENSE), Npgsql | The pinned license preserves the copyright and license paragraphs in copies. |
| Pomelo EF Core provider 9.0.0 | [MIT, LICENSE](https://github.com/PomeloFoundation/Pomelo.EntityFrameworkCore.MySql/blob/58dd6883cb9616fe49f671b943a826b459c0117f/LICENSE), Pomelo Foundation | Review source headers and preserve the upstream copyright and permission notice where required. |

The [source lock](docs/upstream/lock.json) records commit and license blob IDs. For any copied or adapted material or approved dependency, record upstream repository, exact commit, file and symbol in a reviewed ADR. Check the exact headers, terms and relevant notices, and update distribution attributions before release. GoEF's own license decision is tracked in [#15](https://github.com/igorpooh1978/GoEF/issues/15) and the release workflow.

## W002 test module dependencies

The isolated W002 test module pins pgx v5.13.0 (MIT), modernc.org/sqlite v1.63.0 (BSD-3-Clause), go-mssqldb v1.11.0 (BSD-3-Clause), and go-sql-driver/mysql v1.10.0 (MPL-2.0). The selected modules include their license texts. See the [dependency and checksum report](docs/evidence/W002/DRIVER_GHERKIN.md). Before distributing a test binary, review the resolved transitive graph and preserve its applicable license terms. The core Go module does not import the test drivers.

The older SQLite test build separately pins modernc.org/sqlite v1.47.0. Its [tagged license](https://raw.githubusercontent.com/modernc-org/sqlite/v1.47.0/LICENSE) is BSD-3-Clause.
