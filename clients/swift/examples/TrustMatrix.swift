import Foundation
import Quidnug

/// Trust-matrix example — a SwiftUI view model that renders an N×N
/// relational-trust grid for a small org chart.
///
/// Useful for internal trust-ops dashboards, collaborative reputation
/// visualizations, or research notebooks.
@MainActor
final class TrustMatrixViewModel: ObservableObject {
    @Published var matrix: [[Double]] = []
    @Published var loading = false
    @Published var error: String?

    private let client: QuidnugClient
    private let quids: [String]
    private let domain: String

    init(client: QuidnugClient, quids: [String], domain: String) {
        self.client = client
        self.quids = quids
        self.domain = domain
    }

    func load() async {
        loading = true
        defer { loading = false }
        var result = Array(repeating: Array(repeating: 0.0, count: quids.count),
                           count: quids.count)

        await withTaskGroup(of: (Int, Int, Double).self) { group in
            for (i, observer) in quids.enumerated() {
                for (j, target) in quids.enumerated() where i != j {
                    group.addTask {
                        do {
                            let tr = try await self.client.getTrust(
                                observer: observer, target: target,
                                domain: self.domain)
                            return (i, j, tr.trustLevel)
                        } catch {
                            return (i, j, 0.0)
                        }
                    }
                }
            }
            for await (i, j, score) in group {
                result[i][j] = score
            }
        }

        matrix = result
    }
}
