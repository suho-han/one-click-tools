import Foundation
import XCTest
@testable import OctMenubarApp

final class UsageSnapshotTests: XCTestCase {
    func testUsageSnapshotProjectionFromCLIResponse() throws {
        let json = #"""
        {
          "summary": {
            "total": 2,
            "ok": 1,
            "warn": 1,
            "error": 0
          },
          "results": [
            {
              "provider": "codex",
              "status": "ok",
              "used": "63.0",
              "unit": "percent",
              "buckets": {
                "5h": "63.0",
                "7d": "35.0"
              },
              "message": "Usage extracted from local Codex session logs"
            },
            {
              "provider": "opencode",
              "status": "warn",
              "used": "0",
              "unit": "percent",
              "message": "No data: No local OpenCode session logs found"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let date = Date(timeIntervalSince1970: 1_781_284_364)
        let snapshot = UsageSnapshot.from(
            response: response,
            refreshDate: date,
            refreshInterval: 60,
            usageDisplayMode: .used,
            timeZone: TimeZone(secondsFromGMT: 9 * 60 * 60)!
        )

        XCTAssertEqual(snapshot.statusItemTitle, "oct")
        XCTAssertEqual(snapshot.summaryLine, "2 providers · 1 ok · 1 warn · 0 error")
        XCTAssertEqual(snapshot.lastRefreshLabel, "02:12:44")
        XCTAssertEqual(snapshot.nextRefreshLabel, "02:13:44")
        XCTAssertEqual(snapshot.autoRefreshLabel, "Auto refresh: every 1m")
        XCTAssertEqual(snapshot.providers.count, 2)
        XCTAssertEqual(snapshot.providers[0], ProviderCard(name: "codex", status: .ok, metrics: [.init(label: "7d", value: "35.0%")], message: "Usage extracted from local Codex session logs"))
        XCTAssertEqual(snapshot.providers[1], ProviderCard(name: "opencode", status: .warn, metrics: [], message: "No data: No local OpenCode session logs found"))
    }

    func testUsageSnapshotSurfacesOpenCodeMonthlyBucket() throws {
        // Regression test: visibleMetrics() used to only look at "5h"/"7d",
        // so OpenCode Go's monthly quota (bucket "1m") never rendered even
        // though `oct usage --json` reports it.
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "opencode",
              "status": "ok",
              "used": "2",
              "unit": "percent",
              "buckets": {
                "5h": "2",
                "7d": "84",
                "1m": "42"
              },
              "message": "Fetched from OpenCode Go API"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, usageDisplayMode: .used)

        XCTAssertEqual(
            snapshot.providers[0].metrics,
            [.init(label: "5h", value: "2%"), .init(label: "7d", value: "84%"), .init(label: "1m", value: "42%")]
        )
    }

    func testUsageSnapshotSupportsCompactStatusItemTitle() throws {
        let json = #"""
        {
          "summary": {
            "total": 3,
            "ok": 3,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "claude-code",
              "status": "ok",
              "used": "55.0",
              "unit": "percent",
              "buckets": {
                "5h": "55.0"
              }
            },
            {
              "provider": "codex",
              "status": "ok",
              "used": "80.0",
              "unit": "percent",
              "buckets": {
                "7d": "75.0"
              }
            },
            {
              "provider": "commandcode",
              "status": "ok",
              "used": "73.3",
              "unit": "percent",
              "buckets": {
                "5h": "73.3",
                "7d": "31.3",
                "1m": "15.6"
              }
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, titleMode: .compact, usageDisplayMode: .remaining)

        XCTAssertEqual(snapshot.statusItemTitle, "C-45% X-25% D-27%")
        XCTAssertTrue(snapshot.statusItemAccessibilityLabel.contains("claude-code 5h 45.0% left"))
        XCTAssertTrue(snapshot.statusItemAccessibilityLabel.contains("codex 7d 25.0% left"))
        XCTAssertTrue(snapshot.statusItemAccessibilityLabel.contains("commandcode 5h 26.7% left"))

        // The same numbers, read from the same metrics, must appear in the
        // popover card strip below the title -- this is the regression guard
        // for the bug where the compact title always showed "remaining"
        // while the card body always showed the raw "used" value beneath it.
        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "5h", value: "45.0% left")])
        XCTAssertEqual(snapshot.providers[1].metrics, [.init(label: "7d", value: "25.0% left")])
        XCTAssertEqual(
            snapshot.providers[2].metrics,
            [.init(label: "5h", value: "26.7% left"), .init(label: "7d", value: "68.7% left"), .init(label: "1m", value: "84.4% left")]
        )
    }

    func testUsageSnapshotCompactStatusItemTitleHonorsUsedMode() throws {
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "claude-code",
              "status": "ok",
              "used": "55.0",
              "unit": "percent",
              "buckets": {
                "5h": "55.0"
              }
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, titleMode: .compact, usageDisplayMode: .used)

        // Regression guard for the legacy menubar-style bug: the compact
        // title used to always show "remaining" regardless of the configured
        // mode. In "used" mode the title and card body must both show 55%,
        // not the remaining 45%.
        XCTAssertEqual(snapshot.statusItemTitle, "C-55%")
        XCTAssertTrue(snapshot.statusItemAccessibilityLabel.contains("claude-code 5h 55.0%"))
        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "5h", value: "55.0%")])
    }

    func testUsageSnapshotSummaryUsesProjectedProviderStatuses() throws {
        let json = #"""
        {
          "summary": {
            "total": 2,
            "ok": 2,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "claude-code",
              "status": "ok",
              "used": "n/a",
              "unit": "percent",
              "message": "No Claude OAuth token found"
            },
            {
              "provider": "codex",
              "status": "ok",
              "used": "14.0",
              "unit": "percent",
              "buckets": {
                "5h": "14.0",
                "7d": "13.0"
              },
              "message": "Usage extracted from local Codex session logs"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)

        XCTAssertEqual(snapshot.summaryLine, "2 providers · 1 ok · 1 warn · 0 error")
        XCTAssertEqual(snapshot.statusItemTitle, "oct")
        XCTAssertEqual(snapshot.providers.map(\.status), [.warn, .ok])
    }

    func testUsageSnapshotHidesUnknownMetricBuckets() throws {
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "codex",
              "status": "ok",
              "used": "2.0",
              "unit": "percent",
              "buckets": {
                "7d": "2.0"
              },
              "message": "Usage fetched from Codex backend API (weekly bucket)"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, usageDisplayMode: .used)

        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "7d", value: "2.0%")])
    }

    func testUsageSnapshotShowsLegacyCodexFiveHourBucketWhenWeeklyMissing() throws {
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "codex",
              "status": "ok",
              "used": "4.0",
              "unit": "percent",
              "buckets": {
                "5h": "4.0"
              },
              "message": "Usage extracted from local Codex session logs"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, usageDisplayMode: .used)

        XCTAssertEqual(snapshot.providers[0].metrics, [.init(label: "5h", value: "4.0%")])
    }

    func testUsageSnapshotSurfacesModelBucketsWhenNoTimeWindowPresent() throws {
        // Regression test: Antigravity/Gemini keys its buckets "model:<name>"
        // instead of "5h"/"7d"/"1m", so visibleMetrics used to return zero
        // metrics for it even though `oct usage` (the Go table) shows them
        // via its own model-bucket fallback.
        let json = #"""
        {
          "summary": { "total": 1, "ok": 1, "warn": 0, "error": 0 },
          "results": [
            {
              "provider": "antigravity",
              "status": "ok",
              "used": "12.4",
              "unit": "percent",
              "buckets": { "model:gemini-2.5-pro": "12.4" },
              "message": "Usage fetched from Google Code Assist quota API"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let usedSnapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, usageDisplayMode: .used)
        XCTAssertEqual(usedSnapshot.providers[0].metrics, [.init(label: "gemini-2.5-pro", value: "12.4%")])

        let remainingSnapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, usageDisplayMode: .remaining)
        XCTAssertEqual(remainingSnapshot.providers[0].metrics, [.init(label: "gemini-2.5-pro", value: "87.6% left")])
    }

    func testUsageSnapshotSurfacesQuotaBucketWhenNoTimeWindowPresent() throws {
        // Regression test: Copilot keys its pre-computed percentage as a
        // single "quota" bucket (unit is "AIC", a count, not "percent"), so
        // visibleMetrics used to return zero metrics for it.
        let json = #"""
        {
          "summary": { "total": 1, "ok": 1, "warn": 0, "error": 0 },
          "results": [
            {
              "provider": "copilot",
              "status": "ok",
              "used": "117",
              "unit": "AIC",
              "buckets": { "quota": "58.3" },
              "message": "Usage fetched from GitHub Copilot quota API"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let usedSnapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, usageDisplayMode: .used)
        XCTAssertEqual(usedSnapshot.providers[0].metrics, [.init(label: "quota", value: "58.3%")])

        let remainingSnapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60, usageDisplayMode: .remaining)
        XCTAssertEqual(remainingSnapshot.providers[0].metrics, [.init(label: "quota", value: "41.7% left")])
    }

    func testUsageSnapshotKeepsPlanOutOfProviderMessage() throws {
        let json = #"""
        {
          "summary": {
            "total": 1,
            "ok": 1,
            "warn": 0,
            "error": 0
          },
          "results": [
            {
              "provider": "codex",
              "plan": "plus",
              "plan_source": "local logs",
              "status": "ok",
              "used": "15.0",
              "unit": "percent",
              "buckets": {
                "5h": "15.0",
                "7d": "10.0"
              },
              "message": "Usage extracted from local Codex session logs"
            }
          ]
        }
        """#

        let response = try JSONDecoder().decode(UsageResponse.self, from: Data(json.utf8))
        let snapshot = UsageSnapshot.from(response: response, refreshDate: .now, refreshInterval: 60)

        XCTAssertEqual(snapshot.providers[0].plan, "plus")
        XCTAssertEqual(snapshot.providers[0].message, "Usage extracted from local Codex session logs")
        XCTAssertFalse(snapshot.providers[0].message?.contains("Plan:") ?? true)
    }

    func testConfigurationSnapshotDecodesConfigListJSON() throws {
        let json = #"""
        {
          "config_file": "/Users/me/.oct/config.yaml",
          "usage_display_mode": "remaining",
          "menubar_title_mode": "compact",
          "session_refresh_enabled": true,
          "session_refresh_interval": "weekly",
          "session_refresh_hour": 9,
          "tools": [
            {
              "name": "OpenAI Codex",
              "binary_name": "codex",
              "enabled": true
            },
            {
              "name": "Command Code",
              "binary_name": "commandcode",
              "enabled": true
            },
            {
              "name": "Claude Code",
              "binary_name": "claude",
              "enabled": false
            }
          ]
        }
        """#

        let snapshot = try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(json.utf8))

        XCTAssertEqual(snapshot.configFile, "/Users/me/.oct/config.yaml")
        XCTAssertEqual(snapshot.usageDisplayMode, .remaining)
        XCTAssertEqual(snapshot.menubarTitleMode, .compact)
        XCTAssertTrue(snapshot.sessionRefreshEnabled)
        XCTAssertEqual(snapshot.sessionRefreshInterval, "weekly")
        XCTAssertEqual(snapshot.sessionRefreshHour, 9)
        XCTAssertEqual(snapshot.tools.map(\.binaryName), ["codex", "commandcode", "claude"])
        XCTAssertEqual(snapshot.tools.map(\.enabled), [true, true, false])
    }

    func testConfigurationSnapshotDecodesLegacyConfigListJSON() throws {
        let json = #"""
        {
          "config_file": "/Users/me/.oct/config.yaml",
          "usage_display_mode": "remaining",
          "session_refresh_enabled": true,
          "session_refresh_interval": "weekly",
          "session_refresh_hour": 9,
          "tools": [
            {
              "name": "OpenAI Codex",
              "binary_name": "codex",
              "enabled": true
            }
          ]
        }
        """#

        let snapshot = try JSONDecoder().decode(ConfigurationSnapshot.self, from: Data(json.utf8))

        XCTAssertEqual(snapshot.configFile, "/Users/me/.oct/config.yaml")
        XCTAssertEqual(snapshot.usageDisplayMode, .remaining)
        XCTAssertEqual(snapshot.menubarTitleMode, .oct)
        XCTAssertEqual(snapshot.tools.map(\.binaryName), ["codex"])
    }

    func testConfigurationDraftBuildsUpdatePayloadAfterEdits() throws {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/config.yaml",
            usageDisplayMode: .remaining,
            menubarTitleMode: .oct,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true),
                ConfigTool(name: "Command Code", binaryName: "commandcode", enabled: false),
                ConfigTool(name: "Claude Code", binaryName: "claude", enabled: false),
            ]
        )
        var draft = ConfigurationDraft(snapshot: snapshot)

        draft.setTool("claude", enabled: true)
        draft.setTool("commandcode", enabled: true)
        draft.usageDisplayMode = .used
        draft.sessionRefreshEnabled = true
        draft.sessionRefreshInterval = "weekly"
        draft.sessionRefreshHour = 22
        draft.moveTool("claude", by: -1)

        let payload = draft.updatePayload()

        XCTAssertEqual(payload.enabledTools, ["codex", "claude", "commandcode"])
        XCTAssertEqual(payload.usageDisplayMode, .used)
        XCTAssertEqual(payload.menubarTitleMode, .oct)
        XCTAssertTrue(payload.sessionRefreshEnabled)
        XCTAssertEqual(payload.sessionRefreshInterval, "weekly")
        XCTAssertEqual(payload.sessionRefreshHour, 22)
        XCTAssertEqual(payload.agentOrder, ["codex", "claude", "commandcode"])
    }

    func testConfigurationDraftRevertsToLoadedSnapshot() {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/config.yaml",
            usageDisplayMode: .remaining,
            menubarTitleMode: .compact,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true),
            ]
        )
        var draft = ConfigurationDraft(snapshot: snapshot)
        draft.usageDisplayMode = .used
        draft.menubarTitleMode = .oct
        draft.setTool("codex", enabled: false)

        draft.revert(to: snapshot)

        XCTAssertEqual(draft.usageDisplayMode, .remaining)
        XCTAssertEqual(draft.menubarTitleMode, .compact)
        XCTAssertEqual(draft.tools.map(\.enabled), [true])
    }

    func testConfigurationDraftPersistsMenubarTitleModeSelection() throws {
        let snapshot = ConfigurationSnapshot(
            configFile: "/tmp/config.yaml",
            usageDisplayMode: .remaining,
            menubarTitleMode: .oct,
            sessionRefreshEnabled: false,
            sessionRefreshInterval: "daily",
            sessionRefreshHour: 9,
            tools: [
                ConfigTool(name: "OpenAI Codex", binaryName: "codex", enabled: true),
            ]
        )
        var draft = ConfigurationDraft(snapshot: snapshot)

        draft.setMenubarTitleMode(.compact)
        let payload = draft.updatePayload()
        let encoded = try JSONEncoder().encode(payload)
        let decoded = try JSONDecoder().decode(ConfigurationUpdatePayload.self, from: encoded)

        XCTAssertEqual(draft.menubarTitleMode, .compact)
        XCTAssertEqual(decoded.menubarTitleMode, .compact)
        XCTAssertEqual(MenubarTitleMode.compact.label, "Compact %")
    }

    func testResolveExecutablePrefersExplicitOverride() throws {
        let temp = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent(UUID().uuidString)
        let override = temp.appendingPathComponent("custom-oct")
        try FileManager.default.createDirectory(at: temp, withIntermediateDirectories: true)
        FileManager.default.createFile(atPath: override.path, contents: Data())
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: override.path)

        let resolution = OctCLIService.resolveExecutable(
            environment: ["OCT_MENUBAR_OCT_PATH": override.path],
            currentDirectoryURL: temp,
            processExecutableURL: temp.appendingPathComponent("OctMenubarApp")
        )

        XCTAssertEqual(resolution.url.standardizedFileURL.path, override.standardizedFileURL.path)
    }

    func testResolveExecutableWalksAncestorDirectories() throws {
        let temp = URL(fileURLWithPath: NSTemporaryDirectory()).appendingPathComponent(UUID().uuidString)
        let repoRoot = temp.appendingPathComponent("repo")
        let workingDir = repoRoot.appendingPathComponent("macos/OctMenubar")
        let oct = repoRoot.appendingPathComponent("oct")
        try FileManager.default.createDirectory(at: workingDir, withIntermediateDirectories: true)
        FileManager.default.createFile(atPath: oct.path, contents: Data())
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: oct.path)

        let resolution = OctCLIService.resolveExecutable(
            environment: [:],
            currentDirectoryURL: workingDir,
            processExecutableURL: workingDir.appendingPathComponent(".build/debug/OctMenubarApp")
        )

        XCTAssertEqual(resolution.url.standardizedFileURL.path, oct.standardizedFileURL.path)
        XCTAssertTrue(resolution.searchedPaths.contains(oct.standardizedFileURL.path))
    }

    @MainActor
    func testPopoverPreferredSizeCapsHeightForScrollableContent() {
        let compact = PopoverView.preferredSize(for: 2)
        XCTAssertEqual(compact.width, 640)
        XCTAssertLessThan(compact.height, 620)

        let crowded = PopoverView.preferredSize(for: 12)
        XCTAssertEqual(crowded.width, 640)
        XCTAssertEqual(crowded.height, 620)
    }

    func testSettingsActionUsesSwiftUISettingsLink() throws {
        let testFile = URL(fileURLWithPath: #filePath)
        let packageRoot = testFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let footerPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/Views/FooterActionsView.swift")
        let popoverPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/PopoverView.swift")
        let footerSource = try String(contentsOf: footerPath, encoding: .utf8)
        let popoverSource = try String(contentsOf: popoverPath, encoding: .utf8)

        XCTAssertTrue(footerSource.contains("SettingsLink"), "Settings action should use SwiftUI SettingsLink")
        XCTAssertFalse(popoverSource.contains("showSettingsWindow"), "Settings action should not rely on AppKit showSettingsWindow selector")
    }

    func testProviderCardShowsPlanInHeaderWithoutPlanStrip() throws {
        let testFile = URL(fileURLWithPath: #filePath)
        let packageRoot = testFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let providerPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/Views/ProviderCardView.swift")
        let source = try String(contentsOf: providerPath, encoding: .utf8)

        XCTAssertTrue(source.contains("provider.plan"), "Provider card should still render the plan value")
        XCTAssertFalse(source.contains("Text(\"PLAN\")"), "Provider card should not render the old full-width PLAN label")
        XCTAssertFalse(source.contains("private var planStrip"), "Provider card should remove the old full-width plan strip")
    }

    func testRefreshMetadataRendersBelowProviderSection() throws {
        let testFile = URL(fileURLWithPath: #filePath)
        let packageRoot = testFile
            .deletingLastPathComponent()
            .deletingLastPathComponent()
            .deletingLastPathComponent()
        let headerPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/Views/HeaderView.swift")
        let popoverPath = packageRoot.appendingPathComponent("Sources/OctMenubarApp/PopoverView.swift")
        let headerSource = try String(contentsOf: headerPath, encoding: .utf8)
        let popoverSource = try String(contentsOf: popoverPath, encoding: .utf8)

        XCTAssertFalse(headerSource.contains("Last refresh"), "Header should not own refresh metadata")
        XCTAssertTrue(popoverSource.contains("refreshMetadataSection"), "Popover should render refresh metadata below provider usage")
        XCTAssertLessThan(
            popoverSource.range(of: "providerSection")!.lowerBound,
            popoverSource.range(of: "refreshMetadataSection")!.lowerBound
        )
        XCTAssertLessThan(
            popoverSource.range(of: "refreshMetadataSection")!.lowerBound,
            popoverSource.range(of: "FooterActionsView")!.lowerBound
        )
    }
}
