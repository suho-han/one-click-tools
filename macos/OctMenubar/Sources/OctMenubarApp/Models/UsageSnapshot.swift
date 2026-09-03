import Foundation
import SwiftUI

struct UsageSnapshot: Equatable {
    let statusItemTitle: String
    let statusItemAccessibilityLabel: String
    let title: String
    let summaryLine: String
    let lastRefreshLabel: String
    let nextRefreshLabel: String
    let autoRefreshLabel: String
    let providers: [ProviderCard]
    let note: String?

    static let placeholder = UsageSnapshot(
        statusItemTitle: "oct",
        statusItemAccessibilityLabel: "oct usage loading",
        title: "Usage Overview",
        summaryLine: "Loading usage…",
        lastRefreshLabel: "-",
        nextRefreshLabel: "pending",
        autoRefreshLabel: "Auto refresh: every 1m",
        providers: [
            ProviderCard(name: "codex", status: .loading, metrics: [], message: "Waiting for first refresh"),
            ProviderCard(name: "claude-code", status: .loading, metrics: [], message: "Refresh timer starts after launch"),
            ProviderCard(name: "commandcode", status: .loading, metrics: [], message: "Billing API usage loads after refresh"),
        ],
        note: "Data will be loaded from oct usage --json after launch."
    )

    static func error(message: String, refreshInterval: TimeInterval = 60) -> UsageSnapshot {
        UsageSnapshot(
            statusItemTitle: "oct",
            statusItemAccessibilityLabel: "oct usage refresh failed",
            title: "Usage Overview",
            summaryLine: "Refresh failed",
            lastRefreshLabel: "-",
            nextRefreshLabel: "pending",
            autoRefreshLabel: "Auto refresh: every \(DurationFormatter.label(for: refreshInterval))",
            providers: [
                ProviderCard(name: "oct", status: .error, metrics: [], message: message),
            ],
            note: "Check oct path or run Refresh now after fixing the CLI location."
        )
    }
}

struct ProviderCard: Equatable, Identifiable {
    let id: String
    let name: String
    let plan: String
    let planSource: String?
    let status: ProviderStatus
    let metrics: [UsageMetric]
    let message: String?

    init(name: String, plan: String = "unknown", planSource: String? = nil, status: ProviderStatus, metrics: [UsageMetric], message: String?) {
        self.id = name
        self.name = name
        self.plan = plan
        self.planSource = planSource
        self.status = status
        self.metrics = metrics
        self.message = message
    }

    var compactLabel: String {
        let provider = name.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        if provider.contains("claude") {
            return "C"
        }
        if provider.contains("commandcode") || provider.contains("command code") {
            return "D"
        }
        if provider.contains("codex") || provider.contains("openai") {
            return "X"
        }
        if provider.contains("antigravity") || provider.contains("gemini") || provider == "agy" {
            return "G"
        }
        if provider.contains("cursor") {
            return "R"
        }
        if provider.contains("copilot") || provider.contains("github") {
            return "P"
        }
        if provider.contains("opencode") {
            return "O"
        }
        return provider.first.map { String($0).uppercased() } ?? "?"
    }

    // metrics[0].value is already resolved for the configured usage display
    // mode (see UsageSnapshot.visibleMetrics), so this only reformats it as a
    // rounded compact percentage -- it must NOT re-invert used/remaining, or
    // "remaining" mode would double-invert back to a used value while still
    // labeled "remaining" (the exact bug this file fixes).
    var compactMetricValue: String {
        guard let metric = metrics.first else {
            return "?"
        }
        let raw = metric.value
            .replacingOccurrences(of: "% left", with: "")
            .replacingOccurrences(of: "%", with: "")
            .trimmingCharacters(in: .whitespacesAndNewlines)
        guard let value = Double(raw) else {
            return "?"
        }
        return "\(Int(value.rounded()))%"
    }

    var compactAccessibilityLabel: String {
        guard let metric = metrics.first else {
            return "\(name) usage unavailable"
        }
        return "\(name) \(metric.label) \(metric.value)"
    }

    func accentColor(useProviderAccentColors: Bool) -> Color {
        guard useProviderAccentColors else {
            return .primary
        }

        switch name.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
        case "claude-code":
            return Color(red: 0.93, green: 0.56, blue: 0.31)
        case "commandcode":
            return Color(red: 0.38, green: 0.65, blue: 0.98)
        case "codex":
            return Color(red: 0.36, green: 0.71, blue: 0.98)
        case "antigravity":
            return Color(red: 0.58, green: 0.49, blue: 0.96)
        case "copilot":
            return Color(red: 0.40, green: 0.85, blue: 0.73)
        case "cursor":
            return Color(red: 0.99, green: 0.66, blue: 0.25)
        case "opencode":
            return Color(red: 0.97, green: 0.44, blue: 0.60)
        default:
            return .accentColor
        }
    }

}

struct UsageMetric: Equatable {
    let label: String
    let value: String
}

enum ProviderStatus: String, Decodable, Equatable {
    case ok
    case warn
    case error
    case loading

    var badgeLabel: String {
        rawValue.uppercased()
    }

    var tint: Color {
        switch self {
        case .ok:
            return Color(nsColor: .systemGreen)
        case .warn:
            return Color(nsColor: .systemOrange)
        case .error:
            return Color(nsColor: .systemRed)
        case .loading:
            return Color(nsColor: .systemGray)
        }
    }

    var softBackground: Color {
        tint.opacity(0.14)
    }
}

struct UsageResponse: Decodable, Equatable {
    struct Summary: Decodable, Equatable {
        let total: Int
        let ok: Int
        let warn: Int
        let error: Int
    }

    struct Result: Decodable, Equatable {
        let provider: String
        let plan: String?
        let planSource: String?
        let status: String
        let used: String
        let unit: String
        let buckets: [String: String]?
        let message: String?

        enum CodingKeys: String, CodingKey {
            case provider
            case plan
            case planSource = "plan_source"
            case status
            case used
            case unit
            case buckets
            case message
        }
    }

    let summary: Summary
    let results: [Result]
}

extension UsageSnapshot {
    static func from(
        response: UsageResponse,
        refreshDate: Date,
        refreshInterval: TimeInterval,
        titleMode: MenubarTitleMode = .oct,
        usageDisplayMode: UsageDisplayMode = .remaining,
        timeZone: TimeZone = .autoupdatingCurrent
    ) -> UsageSnapshot {
        let formatter = DateFormatter()
        formatter.locale = Locale(identifier: "en_US_POSIX")
        formatter.timeZone = timeZone
        formatter.dateFormat = "HH:mm:ss"

        let providers = response.results.map { result in
            ProviderCard(
                name: result.provider,
                plan: normalizedPlan(result.plan),
                planSource: normalizedPlanSource(result.planSource),
                status: effectiveStatus(for: result),
                metrics: visibleMetrics(for: result.provider, unit: result.unit, buckets: result.buckets, mode: usageDisplayMode),
                message: composedMessage(for: result)
            )
        }
        let summary = projectedSummary(for: providers)
        let status = aggregateStatus(summary: summary)
        let statusItem = statusItemPresentation(for: status, providers: providers, titleMode: titleMode, usageDisplayMode: usageDisplayMode)
        return UsageSnapshot(
            statusItemTitle: statusItem.title,
            statusItemAccessibilityLabel: statusItem.accessibilityLabel,
            title: "Usage Overview",
            summaryLine: "\(summary.total) providers · \(summary.ok) ok · \(summary.warn) warn · \(summary.error) error",
            lastRefreshLabel: formatter.string(from: refreshDate),
            nextRefreshLabel: formatter.string(from: refreshDate.addingTimeInterval(refreshInterval)),
            autoRefreshLabel: "Auto refresh: every \(DurationFormatter.label(for: refreshInterval))",
            providers: providers,
            note: status == .error ? "One or more providers need attention." : "Data comes from oct usage --json without submitting prompts."
        )
    }

    private static func aggregateStatus(summary: UsageResponse.Summary) -> ProviderStatus {
        if summary.error > 0 {
            return .error
        }
        if summary.warn > 0 {
            return .warn
        }
        return .ok
    }

    private static func projectedSummary(for providers: [ProviderCard]) -> UsageResponse.Summary {
        UsageResponse.Summary(
            total: providers.count,
            ok: providers.filter { $0.status == .ok }.count,
            warn: providers.filter { $0.status == .warn }.count,
            error: providers.filter { $0.status == .error }.count
        )
    }

    private static func statusItemPresentation(
        for status: ProviderStatus,
        providers: [ProviderCard],
        titleMode: MenubarTitleMode,
        usageDisplayMode: UsageDisplayMode
    ) -> (title: String, accessibilityLabel: String) {
        guard titleMode == .compact else {
            return ("oct", "oct usage \(status.rawValue)")
        }
        let parts = providers.map { "\($0.compactLabel)-\($0.compactMetricValue)" }
        let title = parts.joined(separator: " ").trimmingCharacters(in: .whitespacesAndNewlines)
        guard !title.isEmpty else {
            return ("oct", "oct usage unavailable")
        }
        let accessibilityParts = providers.map(\.compactAccessibilityLabel)
        let prefix = usageDisplayMode == .remaining ? "Remaining usage" : "Used"
        return (title, "\(prefix): \(accessibilityParts.joined(separator: ", "))")
    }

    private static func classifyStatus(_ raw: String) -> String {
        switch raw.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() {
        case "", "ok":
            return "ok"
        case "warn":
            return "warn"
        case "loading":
            return "loading"
        default:
            return "error"
        }
    }

    private static func effectiveStatus(for result: UsageResponse.Result) -> ProviderStatus {
        let normalized = classifyStatus(result.status)
        guard normalized == "ok" else {
            return ProviderStatus(rawValue: normalized) ?? .error
        }

        let used = result.used.trimmingCharacters(in: .whitespacesAndNewlines).lowercased()
        if used.isEmpty || used == "n/a" {
            return .warn
        }

        let message = result.message?.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() ?? ""
        if hasNoDataSignal(message) || hasPartialSignal(message) {
            return .warn
        }

        return .ok
    }

    private static func hasNoDataSignal(_ message: String) -> Bool {
        message.hasPrefix("no data:") ||
        message.hasPrefix("no ") ||
        message.contains("not found") ||
        message.contains("no configured") ||
        message.contains("no usage metrics")
    }

    private static func hasPartialSignal(_ message: String) -> Bool {
        message.hasPrefix("partial:") || message.contains("partial data")
    }

    // visibleMetrics resolves each bucket's value for `mode` here, once, so
    // every consumer (the popover card strip and the compact status-item
    // title, via ProviderCard.compactMetricValue) reads an already-correct
    // number instead of re-deriving used/remaining itself. That mirrors
    // internal/usage.BucketValue on the Go side -- see AGENTS.md.
    private static func visibleMetrics(for provider: String, unit: String?, buckets: [String: String]?, mode: UsageDisplayMode) -> [UsageMetric] {
        guard let buckets else {
            return []
        }

        let isPercent = (unit ?? "").trimmingCharacters(in: .whitespacesAndNewlines).lowercased() == "percent"

        let timeLabels: [String]
        if provider.trimmingCharacters(in: .whitespacesAndNewlines).lowercased() == "codex" {
            timeLabels = visibleMetricValue(buckets["7d"]) == nil ? ["5h"] : ["7d"]
        } else {
            timeLabels = ["5h", "7d", "1m"]
        }

        let timeMetrics: [UsageMetric] = timeLabels.compactMap { label in
            guard let raw = visibleMetricValue(buckets[label]) else {
                return nil
            }
            return UsageMetric(label: label, value: displayValue(raw: raw, isPercent: isPercent, mode: mode))
        }
        if !timeMetrics.isEmpty {
            return timeMetrics
        }

        // No 5h/7d/1m bucket present: fall back to a single pre-computed
        // percentage bucket ("quota", e.g. Copilot) or per-model buckets
        // ("model:x", e.g. Antigravity/Gemini), mirroring the Go usage
        // table's fallback so the popover doesn't render zero metrics for
        // providers that don't key their buckets by time window.
        if let quota = visibleMetricValue(buckets["quota"]) {
            return [UsageMetric(label: "quota", value: displayValue(raw: quota, isPercent: true, mode: mode))]
        }

        let modelKeys = buckets.keys.filter { $0.hasPrefix("model:") }.sorted()
        return modelKeys.compactMap { key in
            guard let raw = visibleMetricValue(buckets[key]) else {
                return nil
            }
            let label = String(key.dropFirst("model:".count))
            return UsageMetric(label: label.isEmpty ? "model" : label, value: displayValue(raw: raw, isPercent: isPercent, mode: mode))
        }
    }

    // displayValue applies the same used/remaining convention as the Go
    // table: "used" mode passes the raw string through unmodified (just adds
    // "%"), "remaining" mode always reformats to one decimal place with a
    // "% left" suffix. Non-percent buckets (request/session counts) are
    // never inverted -- there's no valid "remaining" reading for a bare count.
    private static func displayValue(raw: String, isPercent: Bool, mode: UsageDisplayMode) -> String {
        guard isPercent else {
            return raw
        }
        switch mode {
        case .used:
            return raw + "%"
        case .remaining:
            guard let used = Double(raw) else {
                return raw + "%"
            }
            let remaining = min(max(100 - used, 0), 100)
            return String(format: "%.1f%% left", remaining)
        }
    }

    private static func visibleMetricValue(_ raw: String?) -> String? {
        let value = raw?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        if value.isEmpty || value == "-" || value.lowercased() == "n/a" || value.lowercased() == "unavailable" {
            return nil
        }
        return value
    }

    private static func composedMessage(for result: UsageResponse.Result) -> String? {
        let trimmedMessage = result.message?.trimmingCharacters(in: .whitespacesAndNewlines)
        if let trimmedMessage, !trimmedMessage.isEmpty {
            return trimmedMessage
        }
        return nil
    }

    private static func normalizedPlan(_ raw: String?) -> String {
        let trimmed = raw?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return trimmed.isEmpty ? "unknown" : trimmed
    }

    private static func normalizedPlanSource(_ raw: String?) -> String? {
        let trimmed = raw?.trimmingCharacters(in: .whitespacesAndNewlines) ?? ""
        return trimmed.isEmpty ? nil : trimmed
    }
}

enum DurationFormatter {
    static func label(for interval: TimeInterval) -> String {
        let totalSeconds = max(Int(interval.rounded()), 1)
        let minutes = totalSeconds / 60
        let seconds = totalSeconds % 60
        if minutes > 0 && seconds > 0 {
            return "\(minutes)m\(seconds)s"
        }
        if minutes > 0 {
            return "\(minutes)m"
        }
        return "\(seconds)s"
    }
}
