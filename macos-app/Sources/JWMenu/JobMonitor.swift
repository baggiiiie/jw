import Foundation
import AppKit

struct JWConfig: Codable {
    var jobs: [String: JWJob]
    var history: [HistoryEntry]?
}

struct JWJob: Codable {
    let startTime: Date
    let url: String
    let lastCheckFailed: Bool?

    enum CodingKeys: String, CodingKey {
        case startTime = "start_time"
        case url
        case lastCheckFailed = "last_check_failed"
    }
}

struct HistoryEntry: Codable, Identifiable {
    var id: String { url + finishedTime.description }
    let url: String
    let result: String
    let finishedTime: Date
    let startTime: Date

    enum CodingKeys: String, CodingKey {
        case url, result
        case finishedTime = "finished_time"
        case startTime = "start_time"
    }
}

class JobMonitor: ObservableObject {
    @Published var jobs: [String: JWJob] = [:]
    @Published var history: [HistoryEntry] = []
    @Published var lastUpdated: Date? = nil

    private var timer: Timer?
    private var previousJobURLs: Set<String> = []
    private let configPath: String

    init() {
        let home = FileManager.default.homeDirectoryForCurrentUser.path
        configPath = "\(home)/.jw/monitored_jobs.json"
    }

    func startPolling() {
        loadConfig()
        timer = Timer.scheduledTimer(withTimeInterval: 5, repeats: true) { [weak self] _ in
            self?.loadConfig()
        }
    }

    func loadConfig() {
        guard let data = FileManager.default.contents(atPath: configPath) else {
            DispatchQueue.main.async {
                self.jobs = [:]
                self.history = []
                self.lastUpdated = Date()
            }
            return
        }

        let decoder = JSONDecoder()
        decoder.dateDecodingStrategy = .iso8601

        guard let config = try? decoder.decode(JWConfig.self, from: data) else {
            return
        }

        let newJobURLs = Set(config.jobs.keys)
        let finishedURLs = previousJobURLs.subtracting(newJobURLs)

        let historyEntries = config.history ?? []
        for url in finishedURLs {
            if let entry = historyEntries.first(where: { $0.url == url }) {
                sendNotification(jobURL: url, result: entry.result)
            } else {
                sendNotification(jobURL: url, result: "COMPLETED")
            }
        }

        previousJobURLs = newJobURLs

        DispatchQueue.main.async {
            self.jobs = config.jobs
            self.history = historyEntries.sorted { $0.finishedTime > $1.finishedTime }
            self.lastUpdated = Date()
        }
    }

    private func sendNotification(jobURL: String, result: String) {
        let jobName = extractJobName(from: jobURL)

        let title: String
        switch result {
        case "SUCCESS":
            title = "✅ Jenkins Job Completed"
        case "FAILURE":
            title = "❌ Jenkins Job Failed"
        case "ABORTED":
            title = "⏹ Jenkins Job Aborted"
        default:
            title = "Jenkins Job Finished"
        }

        let script = """
            display notification "\(jobName) — \(result)" with title "\(title)" sound name "Ping"
            """
        let proc = Process()
        proc.executableURL = URL(fileURLWithPath: "/usr/bin/osascript")
        proc.arguments = ["-e", script]
        try? proc.run()
    }

    func extractJobName(from url: String) -> String {
        let parts = url.split(separator: "/")
        if parts.count >= 3 {
            return parts.suffix(3).joined(separator: "/")
        }
        return url
    }
}
