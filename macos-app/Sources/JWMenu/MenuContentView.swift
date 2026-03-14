import SwiftUI

struct MenuContentView: View {
    @ObservedObject var monitor: JobMonitor

    var body: some View {
        VStack(alignment: .leading, spacing: 0) {
            HStack {
                Text("JW Monitor")
                    .font(.headline)
                Spacer()
                if let lastUpdated = monitor.lastUpdated {
                    Text(lastUpdated, style: .time)
                        .font(.caption)
                        .foregroundColor(.secondary)
                }
            }
            .padding(.horizontal, 16)
            .padding(.top, 12)
            .padding(.bottom, 8)

            Divider()

            ScrollView {
                VStack(alignment: .leading, spacing: 0) {
                    sectionHeader("Watching \(monitor.jobs.count) job(s)")

                    if monitor.jobs.isEmpty {
                        Text("No jobs being monitored")
                            .font(.caption)
                            .foregroundColor(.secondary)
                            .padding(.horizontal, 16)
                            .padding(.vertical, 8)
                    } else {
                        ForEach(
                            monitor.jobs.sorted(by: { $0.value.startTime > $1.value.startTime }),
                            id: \.key
                        ) { url, job in
                            JobRow(url: url, job: job, monitor: monitor)
                        }
                    }

                    if !monitor.history.isEmpty {
                        Divider()
                            .padding(.vertical, 4)

                        sectionHeader("History")

                        ForEach(monitor.history) { entry in
                            HistoryRow(entry: entry, monitor: monitor)
                        }
                    }
                }
            }

            Divider()

            HStack {
                Button("Quit") {
                    NSApplication.shared.terminate(nil)
                }
                .buttonStyle(.plain)
                .foregroundColor(.secondary)
                .font(.caption)
                Spacer()
            }
            .padding(.horizontal, 16)
            .padding(.vertical, 8)
        }
        .frame(width: 360)
    }

    private func sectionHeader(_ title: String) -> some View {
        Text(title)
            .font(.subheadline.weight(.semibold))
            .foregroundColor(.secondary)
            .padding(.horizontal, 16)
            .padding(.vertical, 6)
    }
}

struct JobRow: View {
    let url: String
    let job: JWJob
    let monitor: JobMonitor

    var body: some View {
        HStack(spacing: 8) {
            Circle()
                .fill(job.lastCheckFailed == true ? Color.yellow : Color.blue)
                .frame(width: 8, height: 8)

            VStack(alignment: .leading, spacing: 2) {
                Text(monitor.extractJobName(from: url))
                    .font(.system(.body, design: .monospaced))
                    .lineLimit(1)
                    .truncationMode(.head)

                Text("Monitoring for \(formatDuration(since: job.startTime))")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }

            Spacer()

            if let link = URL(string: url) {
                Link(destination: link) {
                    Image(systemName: "arrow.up.right.square")
                        .foregroundColor(.secondary)
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 6)
    }
}

struct HistoryRow: View {
    let entry: HistoryEntry
    let monitor: JobMonitor

    private var resultColor: Color {
        switch entry.result {
        case "SUCCESS": return .green
        case "FAILURE": return .red
        default: return .secondary
        }
    }

    private var resultIcon: String {
        switch entry.result {
        case "SUCCESS": return "checkmark.circle.fill"
        case "FAILURE": return "xmark.circle.fill"
        case "ABORTED": return "stop.circle.fill"
        default: return "questionmark.circle.fill"
        }
    }

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: resultIcon)
                .foregroundColor(resultColor)
                .frame(width: 16)

            VStack(alignment: .leading, spacing: 2) {
                Text(monitor.extractJobName(from: entry.url))
                    .font(.system(.body, design: .monospaced))
                    .lineLimit(1)
                    .truncationMode(.head)

                Text("\(entry.result) · \(formatDuration(since: entry.finishedTime)) ago")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }

            Spacer()

            if let link = URL(string: entry.url) {
                Link(destination: link) {
                    Image(systemName: "arrow.up.right.square")
                        .foregroundColor(.secondary)
                }
            }
        }
        .padding(.horizontal, 16)
        .padding(.vertical, 6)
    }
}

func formatDuration(since date: Date) -> String {
    let elapsed = Date().timeIntervalSince(date)
    let totalMinutes = Int(elapsed / 60)
    let days = totalMinutes / (24 * 60)
    let hours = (totalMinutes % (24 * 60)) / 60
    let mins = totalMinutes % 60

    if days > 0 {
        return "\(days)d \(hours)h \(mins)m"
    }
    if hours > 0 {
        return "\(hours)h \(mins)m"
    }
    return "\(mins)m"
}
