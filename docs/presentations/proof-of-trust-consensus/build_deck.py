"""Build the Proof of Trust vs Consensus deck (~90 slides)."""
import pathlib
import sys

HERE = pathlib.Path(__file__).parent
ASSETS = HERE / "assets"
OUTPUT = HERE / "proof-of-trust-consensus.pptx"

sys.path.insert(0, str(HERE.parent))
from _deck_helpers import (  # noqa: E402
    make_presentation, title_slide, section_divider, content_slide,
    two_col_slide, stat_slide, quote_slide, table_slide, image_slide,
    code_slide, icon_grid_slide, closing_slide,
    add_notes, add_footer,
    TEAL, CORAL, EMERALD, AMBER, TEXT_MUTED,
)
from pptx.util import Inches  # noqa: E402

BRAND = "Quidnug  \u00B7  Proof of Trust vs Consensus"
TOTAL = 90


def _im(prs, title, image, image_h=None, **kw):
    if image_h is not None and not hasattr(image_h, 'emu'):
        # Convert bare float/int to Inches
        image_h = Inches(image_h)
    elif image_h is None:
        image_h = Inches(4.6)
    return image_slide(prs, title, image, image_h=image_h,
                       assets_dir=ASSETS, **kw)


def build():
    prs = make_presentation()

    # ==== OPENING (1-5) ====
    s = title_slide(prs,
        "Proof of Trust vs Nakamoto, BFT, FBA, and DAG Consensus",
        "Why relational trust graphs outperform global consensus for "
        "the 95% of real-world applications that aren't moving billions "
        "of dollars between mutually-untrusting strangers.",
        eyebrow="QUIDNUG  \u00B7  THEORY")
    add_notes(s, [
        "Welcome. This is an opinionated talk backed by primary "
        "sources. 90 slides, about 60 minutes with Q&A.",
        "Primary audience: distributed systems engineers, protocol "
        "designers, CTOs evaluating whether they need a blockchain.",
        "Central thesis: the default assumption that every "
        "distributed system needs global consensus is wrong for the "
        "vast majority of applications.",
        "We are not arguing against Bitcoin or Ethereum. We are "
        "arguing that they solve a narrow problem (money), and most "
        "other applications (reputation, consent, identity, supply "
        "chain) need something different."
    ])
    add_footer(s, 1, TOTAL, BRAND)

    s = stat_slide(prs, "0.5%",
        "of global electricity consumption is burned by Bitcoin's "
        "proof-of-work mining.",
        context="That cost is defensible if you are securing trillions "
                "of dollars. It is not defensible for reviews, consent, "
                "or supply chain. We need a different substrate for "
                "those.",
        title="Where we are in 2026")
    add_notes(s, [
        "Anchor the urgency. Cambridge Centre for Alternative Finance "
        "CBECI data puts Bitcoin at ~155 TWh/year in 2024, which is "
        "approximately 0.5% of global electricity use.",
        "This is worth the energy only if you are securing "
        "something genuinely valuable at a global permissionless "
        "scale. For almost every non-money application, the cost is "
        "absurd."
    ])
    add_footer(s, 2, TOTAL, BRAND)

    s = content_slide(prs, "Agenda", bullets=[
        ("Section 1. The consensus problem.",
         "Byzantine Generals, FLP, CAP. What we are actually solving."),
        ("Section 2. Tour of existing mechanisms.",
         "PoW, PoS, BFT family, FBA, DAG, PoA. A tour of the taxonomy."),
        ("Section 3. Their shortcomings.",
         "Energy, centralization, finality, global-state, MEV."),
        ("Section 4. Proof of Trust.",
         "Relational, signed, tiered, domain-scoped. Four principles."),
        ("Section 5. The math of relational trust.",
         "Proofs of monotonic decay, Sybil resistance, bounded computation."),
        ("Section 6. Security properties.",
         "Replay, rotation, Byzantine tolerance, TTL revocation."),
        ("Section 7. Real-world use cases.",
         "Where PoT wins, where it doesn't."),
        ("Section 8. Benchmarks and tradeoffs.",
         "Throughput, latency, energy. Honest about limits."),
        ("Section 9. Decision framework.",
         "Picking the right tool for your use case."),
    ])
    add_notes(s, [
        "Walk the audience through the arc. Sections 1 to 3 establish "
        "the problem and failure modes. Sections 4 to 6 present and "
        "defend the solution. Section 7 shows fit. Sections 8 to 9 "
        "are honest positioning.",
        "Total runtime target: 60 minutes with Q&A buffer."
    ])
    add_footer(s, 3, TOTAL, BRAND)

    s = content_slide(prs, "Four things you will take away", bullets=[
        ("Takeaway 1.",
         "Global consensus is a solution looking for a problem in "
         "most enterprise and application-layer contexts."),
        ("Takeaway 2.",
         "Byzantine Fault Tolerance as typically framed (f < n/3) "
         "is over-engineered for non-token networks."),
        ("Takeaway 3.",
         "Relational trust has cleaner mathematical properties than "
         "any global-state scheme, with monotonic decay, bounded "
         "complexity, and per-observer finality."),
        ("Takeaway 4.",
         "For applications in the 'trust, not token' column, Proof "
         "of Trust is 10 to 1000 times more efficient than PoW and "
         "orders of magnitude simpler than BFT."),
    ])
    add_notes(s, [
        "Preview the four anchors. Return to them in Section 9 as the "
        "summary check.",
        "Takeaway 3 is the mathematically load-bearing claim; we will "
        "prove it in Section 5."
    ])
    add_footer(s, 4, TOTAL, BRAND)

    s = content_slide(prs, "Why this talk matters in 2026", bullets=[
        ("Permissionless blockchain ecosystems are mature.",
         "Bitcoin, Ethereum, Solana, Cosmos have settled into stable "
         "design niches."),
        ("Enterprise blockchain pilots have mostly failed.",
         "Hyperledger Fabric, IBM TradeLens (shut down 2023), Quorum "
         "all struggled because the use cases did not need global "
         "consensus."),
        ("Application-layer trust is the next frontier.",
         "Reviews, consent, KYC, supply chain, identity: all "
         "reputation-shaped problems, all poorly served by existing "
         "consensus mechanisms."),
        ("Regulators are paying attention.",
         "EU AI Act, GDPR Article 22, NIST AI RMF require "
         "attribution and auditability that on-chain tokens can't "
         "provide at scale."),
        ("The architectural choice compounds.",
         "Teams picking a substrate today will live with it for a "
         "decade. Choose based on the actual problem."),
    ])
    add_notes(s, [
        "Make the urgency concrete. Each bullet is cited: Ethereum "
        "Merge September 2022, TradeLens shutdown March 2023, EU AI "
        "Act adopted 2024.",
        "The enterprise blockchain failure point is worth pausing on. "
        "The TradeLens post-mortem (Maersk + IBM) is a clean case "
        "study in 'consensus was the wrong tool.'"
    ])
    add_footer(s, 5, TOTAL, BRAND)

    # ==== SECTION 1: The Consensus Problem (6-14) ====
    s = section_divider(prs, 1, "The Consensus Problem",
        "Byzantine Generals, FLP, CAP. Forty years of theory that "
        "shapes every answer.")
    add_notes(s, [
        "Quick theoretical foundation. Most of the audience has seen "
        "these results; we spend a minute each to anchor the "
        "terminology."
    ])
    add_footer(s, 6, TOTAL, BRAND)

    s = content_slide(prs, "Byzantine Generals Problem (Lamport 1982)",
        bullets=[
            ("Formalization of distributed agreement with adversaries.",
             "Lamport, Shostak, Pease: 'The Byzantine Generals Problem,' "
             "ACM TOPLAS 4(3), 1982."),
            ("The setup.",
             "N processes, some of which are Byzantine (can send "
             "arbitrary messages). Can the honest processes agree on "
             "a common plan?"),
            ("The famous result.",
             "Deterministic synchronous BFT requires n >= 3f+1 for "
             "f Byzantine faults with oral messages."),
            ("With signatures, tolerance improves to n >= 2f+1.",
             "Because signed messages cannot be forged, fewer honest "
             "nodes are needed to outvote traitors."),
            ("This result underlies every classical consensus protocol.",
             "PBFT, Tendermint, HotStuff, Algorand all trace back to "
             "this paper."),
        ])
    add_notes(s, [
        "This is the foundational paper. Every audience member who "
        "cites 'f < n/3' is citing this paper whether they know it or "
        "not."
    ])
    add_footer(s, 7, TOTAL, BRAND)

    s = content_slide(prs, "FLP impossibility (1985)", bullets=[
        ("Fischer, Lynch, Paterson: 'Impossibility of Distributed "
         "Consensus with One Faulty Process,' Journal of the ACM, 1985.",
         ""),
        ("The result in one sentence.",
         "No deterministic consensus protocol can tolerate even a "
         "single crash failure in a fully asynchronous system."),
        ("Why it matters.",
         "Any real-world consensus protocol must relax one of the "
         "three FLP assumptions: determinism, async messaging, or "
         "crash tolerance."),
        ("PoW relaxes determinism.",
         "Nakamoto consensus is probabilistic; finality is "
         "statistical."),
        ("PBFT-family protocols relax async.",
         "They assume partial synchrony with timeouts."),
        ("Proof of Trust relaxes global agreement.",
         "Each observer can maintain their own view. Divergence is "
         "acceptable."),
    ])
    add_notes(s, [
        "FLP is the 'you cannot have everything' result of distributed "
        "systems. Every consensus design is a choice of which "
        "limitation to accept.",
        "The PoT angle is the talk's core thesis: by dropping global "
        "agreement, we get useful properties back."
    ])
    add_footer(s, 8, TOTAL, BRAND)

    s = table_slide(prs,
        "The three network models and what each allows",
        [
            ["Network model", "Consensus possible?",
             "Representative protocols"],
            ["Synchronous",
             "Yes, if n >= 3f+1 (BFT bound)",
             "Textbook BFT"],
            ["Partially synchronous",
             "Yes, with timeouts (Dwork-Lynch-Stockmeyer 1988)",
             "PBFT, Tendermint, HotStuff, Paxos"],
            ["Asynchronous",
             "Impossible with determinism + any failures (FLP)",
             "Randomized: Algorand, Avalanche"],
        ],
        subtitle="Every protocol is some choice of model + relaxation.",
        col_widths=[1.5, 2.3, 2.2])
    add_notes(s, [
        "The DLS 1988 paper is the theoretical bridge: assuming "
        "eventual synchrony is what makes real-world protocols "
        "work.",
        "The async row is why Nakamoto had to use probabilistic "
        "finality."
    ])
    add_footer(s, 9, TOTAL, BRAND)

    s = content_slide(prs, "CAP theorem (Brewer 2000, Gilbert-Lynch 2002)",
        bullets=[
            ("Three desirable properties of a distributed system.",
             "Consistency (same view everywhere), Availability "
             "(every request gets a response), Partition tolerance "
             "(keep working during network splits)."),
            ("The result.",
             "In the presence of network partitions (always true in "
             "practice), you must choose: C or A, not both."),
            ("BFT picks C.",
             "Partition leads to halt. Nothing commits until "
             "quorum returns."),
            ("Nakamoto picks A.",
             "Both sides of the partition keep producing blocks; a "
             "reorg resolves after partition heals."),
            ("Proof of Trust: each observer chooses.",
             "Observers can be on either side of a partition with "
             "their own coherent view. No forced choice globally."),
        ])
    add_notes(s, [
        "CAP is often misunderstood. It is not 'pick 2 of 3'; it is "
        "'partition is inevitable, so pick C or A when it happens.'",
        "The third framing (PoT) rejects the premise that there must "
        "be ONE answer for the whole system."
    ])
    add_footer(s, 10, TOTAL, BRAND)

    s = content_slide(prs, "What real systems actually need", bullets=[
        ("Inventory the distributed systems at any Fortune 500 today.",
         "You find: database replication, Kafka event streams, S3, "
         "ZooKeeper / etcd, Vault, HashiCorp Consul."),
        ("Notice what is absent.",
         "Global-consensus blockchains are almost entirely missing "
         "from production enterprise infrastructure."),
        ("The reason is not ignorance.",
         "Internal trust boundaries (HR, legal, network segmentation) "
         "already constrain who can corrupt state. Byzantine tolerance "
         "is unnecessary overhead."),
        ("The interesting problem is cross-company trust.",
         "Consortium, cross-border, citizen-to-government, reviewer-"
         "to-business: these live outside corporate trust boundaries."),
        ("And the answer there is usually not 'add a blockchain.'",
         "It is usually 'sign records; let each party evaluate trust "
         "in the signer.' That is exactly Proof of Trust."),
    ])
    add_notes(s, [
        "This is the pragmatic reframing. We show engineers the "
        "actual stack they use and ask: what was missing?",
        "The answer is usually: trust across boundaries, not "
        "Byzantine-fault-tolerant state machines."
    ])
    add_footer(s, 11, TOTAL, BRAND)

    s = quote_slide(prs,
        "Global consensus is appropriate for money, where "
        "double-spend prevention is the whole game. It is "
        "inappropriate for reputation, consent, identity, or any "
        "application where different observers legitimately hold "
        "different views.",
        "The central thesis of this talk",
        title="Framing")
    add_notes(s, [
        "Pause on this slide. It is the pivotal framing.",
        "If the audience agrees with this sentence, the next 80 "
        "slides follow. If they disagree, stop and discuss."
    ])
    add_footer(s, 12, TOTAL, BRAND)

    s = content_slide(prs, "Trust-shaped vs money-shaped problems",
        bullets=[
            ("Money is adversarial by default.",
             "Any pair of users may have zero prior trust. Double-spend "
             "prevention requires globally-agreed state."),
            ("Reviews are relational.",
             "Alice weighs Bob's opinion differently than Dave does. "
             "There is no single 'right' aggregate."),
            ("Consent is subject-scoped.",
             "A consent grant from patient P to hospital H has meaning "
             "for those parties and their delegates, not for every "
             "random observer."),
            ("Identity is context-dependent.",
             "Your LinkedIn identity, Twitter identity, and GitHub "
             "identity are linked but not identical. Conflating them "
             "is a loss."),
            ("Supply chain is bilateral.",
             "Supplier S and buyer B have direct trust. Transitive "
             "attestations exist but are always observer-weighted."),
        ])
    add_notes(s, [
        "Taxonomically, this is why the same substrate can't serve "
        "both categories well.",
        "Money gets PoW. Trust-shaped problems get PoT. Mixed "
        "systems use both (see Section 7)."
    ])
    add_footer(s, 13, TOTAL, BRAND)

    s = _im(prs,
        "The consensus family tree",
        "chart_taxonomy.png",
        caption="Stylized taxonomy; some protocols blur categories "
                "(Algorand is both PoS and leaderless BFT).",
        image_h=3.7)
    add_notes(s, [
        "Preview of what we tour next.",
        "Point out that PoT in bottom-left inherits from PGP Web of "
        "Trust (Zimmermann 1991) while adding domain scoping, decay, "
        "and practical revocation."
    ])
    add_footer(s, 14, TOTAL, BRAND)

    # ==== SECTION 2: Tour of Mechanisms (15-32) ====
    s = section_divider(prs, 2, "Tour of Existing Mechanisms",
        "PoW, PoS, BFT, FBA, DAG, PoA. A practical walk through the "
        "landscape.")
    add_footer(s, 15, TOTAL, BRAND)

    s = content_slide(prs, "Proof of Work: the Nakamoto original",
        bullets=[
            ("Satoshi Nakamoto (2008): 'Bitcoin: A Peer-to-Peer "
             "Electronic Cash System.' 8 pages, one of the most "
             "influential computer science papers ever written.",
             ""),
            ("The trick.",
             "Sybil resistance through computational cost. Block "
             "production is a race to find a rare hash."),
            ("Mathematical properties.",
             "Block production follows a Poisson process. Finality "
             "after k confirmations: 1 - (q/p)^k where p is honest "
             "hashrate."),
            ("Why it works.",
             "Economic cost of attack scales with the honest "
             "network's total hashrate, which grows with the value "
             "being protected."),
            ("Why it is limited.",
             "Finality is probabilistic (not instant). Throughput "
             "bounded by block interval and size. Energy cost is "
             "proportional to network value."),
        ])
    add_footer(s, 16, TOTAL, BRAND)

    s = content_slide(prs, "Eyal and Sirer 2014: selfish mining",
        bullets=[
            ("Ittay Eyal and Emin Gun Sirer (Cornell).",
             "'Majority is Not Enough: Bitcoin Mining is Vulnerable,' "
             "Financial Cryptography 2014."),
            ("The attack.",
             "A miner with > 25% of hashrate can profit by "
             "withholding newly-mined blocks. Not 51%: 25%."),
            ("Mechanics.",
             "Keep secret chain ahead of public chain; release "
             "strategically when honest miners catch up."),
            ("Consequences.",
             "The '50% attack' is the wrong threshold. Sustained "
             "deviation from protocol is rational starting at 25%."),
            ("Impact on design.",
             "Every post-2014 consensus design factored in rational-"
             "adversary models. Not just 'honest vs Byzantine' but "
             "'honest vs profit-motivated.'"),
        ])
    add_footer(s, 17, TOTAL, BRAND)

    s = _im(prs, "Annual electricity consumption, real numbers",
        "chart_energy.png",
        caption="Source: Cambridge CBECI, IEA, Ethereum Foundation "
                "post-merge disclosure.",
        image_h=4.4)
    add_notes(s, [
        "Bitcoin's 155 TWh/year is comparable to Poland's total "
        "consumption. Ethereum cut from 78 TWh to 0.01 TWh via the "
        "Merge, a 99.95% reduction.",
        "Quidnug PoT at 0.003 TWh is an estimate: commodity hardware, "
        "signature verify, gossip. Well under the noise floor of "
        "typical data center operation."
    ])
    add_footer(s, 18, TOTAL, BRAND)

    s = content_slide(prs, "Proof of Stake: from PPCoin to Ethereum",
        bullets=[
            ("Core idea.",
             "Replace 'burn electricity' with 'lock up economic "
             "stake.' Attackers lose their stake if caught."),
            ("PPCoin (King and Nadal 2012).",
             "First deployed PoS. Had 'nothing-at-stake' bug; "
             "attackers could cheaply fork without economic penalty."),
            ("Ouroboros (Kiayias et al. 2017, CRYPTO).",
             "First provably-secure PoS protocol. Cardano's basis."),
            ("Algorand (Gilad et al. 2017, SOSP).",
             "Verifiable random functions for leader election; "
             "immediate finality with BFT-style voting."),
            ("Ethereum Gasper (2022 onward).",
             "Two-pronged: block proposals + finality gadget. "
             "Moved 99.95% of Ethereum's energy to 0 post-Merge."),
        ])
    add_footer(s, 19, TOTAL, BRAND)

    s = table_slide(prs, "Proof of Stake variants at a glance", [
        ["Variant", "Finality model", "Representative"],
        ["Chain-based PoS",
         "Probabilistic, longest-chain",
         "Peercoin, NXT (early)"],
        ["BFT-style PoS",
         "Deterministic via 2/3 majority",
         "Tendermint, Cosmos"],
        ["Pure PoS (VRF)",
         "Deterministic after random-leader commit",
         "Algorand"],
        ["Ouroboros",
         "Epoch-based, provably secure",
         "Cardano"],
        ["Gasper / Casper FFG",
         "Slot-based block prop + 2-epoch finality",
         "Ethereum"],
    ],
        col_widths=[1.5, 2.8, 2.0])
    add_footer(s, 20, TOTAL, BRAND)

    s = content_slide(prs, "Classical BFT: PBFT, HotStuff, Tendermint",
        bullets=[
            ("Practical Byzantine Fault Tolerance.",
             "Castro and Liskov 1999: the paper that brought BFT "
             "out of the theory lab into usable code."),
            ("Three-phase protocol.",
             "Pre-prepare, prepare, commit. Each phase requires "
             "2f+1 messages. O(n^2) total message complexity."),
            ("HotStuff (Yin et al. 2019, PODC).",
             "Linear message complexity via threshold signatures. "
             "The protocol underlying Diem, Aptos, Sui."),
            ("Tendermint (Kwon 2014).",
             "Simplified PBFT variant powering Cosmos ecosystem. "
             "Same f < n/3 bound, same O(n^2) messaging."),
            ("Strengths.",
             "Immediate deterministic finality. Sub-second "
             "achievable. Well-studied security properties."),
            ("Weaknesses.",
             "Requires known validators. Scales poorly past ~100 "
             "nodes. Halts under partition."),
        ])
    add_footer(s, 21, TOTAL, BRAND)

    s = content_slide(prs, "Federated Byzantine Agreement (FBA)",
        bullets=[
            ("David Mazieres 2015: 'The Stellar Consensus Protocol.'",
             "The paper that formalized FBA."),
            ("Key difference from PBFT.",
             "Participants choose their own quorum slices rather than "
             "sharing a single global validator set."),
            ("Global agreement emerges from quorum intersection.",
             "Individual slice choices compose into network-wide "
             "consistency when slices overlap correctly."),
            ("Why this is closer to Proof of Trust.",
             "Acknowledges that trust is subjective. Each node picks "
             "who they trust."),
            ("Still not quite PoT.",
             "SCP requires slices to form a 'quorum system with "
             "dispensability,' which in practice requires operators "
             "to coordinate. Binary trust, not graded."),
        ])
    add_footer(s, 22, TOTAL, BRAND)

    s = content_slide(prs, "DAG-based: Tangle, Hashgraph, Avalanche",
        bullets=[
            ("Directed Acyclic Graph replaces linear chain.",
             "Each transaction references multiple prior transactions. "
             "High parallelism, complex ordering semantics."),
            ("IOTA Tangle (Popov 2018).",
             "Each tx validates two prior txs. Historically needed a "
             "central Coordinator; Coordicide work ongoing."),
            ("Hashgraph (Baird 2016).",
             "Gossip-about-gossip plus virtual voting. Elegant, but "
             "patent-encumbered for most of its history."),
            ("Avalanche (Rocket et al. 2018).",
             "Metastable consensus via repeated random sampling. "
             "Probabilistic finality in O(log n) rounds."),
            ("Common claim.",
             "High throughput (10k to 100k TPS) at cost of more "
             "complex finality semantics."),
        ])
    add_footer(s, 23, TOTAL, BRAND)

    s = content_slide(prs, "Proof of Authority: known signers",
        bullets=[
            ("A pre-approved list of validators takes turns producing "
             "blocks.",
             "Examples: Clique (Ethereum EIP-225), IBFT and IBFT 2.0 "
             "in Hyperledger Besu and Quorum."),
            ("Not really 'consensus' in the theoretical sense.",
             "It is coordination among parties who already trust each "
             "other by name."),
            ("Strengths.",
             "Simple, fast, low overhead. Appropriate for internal "
             "consortiums."),
            ("Weaknesses.",
             "Adding or removing validators requires out-of-band "
             "governance. 'Authority' is the whole basis; no "
             "mathematical Sybil resistance."),
            ("Widely used for enterprise blockchain pilots.",
             "And that is why most enterprise blockchain pilots failed "
             "to justify their existence: PoA systems are just "
             "expensive databases."),
        ])
    add_footer(s, 24, TOTAL, BRAND)

    s = content_slide(prs, "PGP Web of Trust: the honorable ancestor",
        bullets=[
            ("Phil Zimmermann 1991: PGP introduced relational trust "
             "to internet-scale identity.",
             ""),
            ("The core idea.",
             "You sign keys you trust. Transitive chains let you "
             "validate keys you have not met directly."),
            ("Why it failed at scale.",
             "No principled decay across hops. No domain scoping "
             "(your web-of-trust for code signing was the same as "
             "for email). No practical revocation."),
            ("What survives.",
             "The model of observer-dependent trust; the idea that "
             "identity is a graph, not a registry."),
            ("Proof of Trust is the PGP WoT vision plus:",
             "Multiplicative decay, domain scoping, TTL, signed "
             "revocations, and a protocol-native aggregation algorithm."),
        ])
    add_footer(s, 25, TOTAL, BRAND)

    # ==== SECTION 3: Shortcomings (26-33) ====
    s = section_divider(prs, 3, "Their Shortcomings",
        "Four structural failures of global-consensus systems for "
        "real-world applications.")
    add_footer(s, 26, TOTAL, BRAND)

    s = content_slide(prs, "Shortcoming 1: energy cost", bullets=[
        ("PoW burns real electricity.",
         "Cambridge CBECI puts Bitcoin at ~155 TWh/year 2024. Around "
         "0.5% of global electricity consumption."),
        ("Defensible for money at global scale.",
         "Bitcoin secures roughly $1T of market cap as of writing. "
         "The cost-per-dollar-protected is small."),
        ("Indefensible for almost every other application.",
         "Tracking reviews, consent grants, or wire authorizations "
         "at 10 MWh per transaction is absurd."),
        ("PoS and BFT close most of this gap.",
         "Ethereum Merge dropped 99.95% of Ethereum's energy "
         "footprint. But they inherited other failure modes."),
        ("This is the ONE advantage PoT does not have to fight for.",
         "PoT has the same energy profile as PBFT: essentially free."),
    ])
    add_footer(s, 27, TOTAL, BRAND)

    s = content_slide(prs,
        "Shortcoming 2: centralization creep",
        bullets=[
            ("Gencer et al. (FC 2018): 'Decentralization in Bitcoin "
             "and Ethereum Networks.'",
             ""),
            ("Actual measurements.",
             "Top 4 Bitcoin mining pools control roughly 53% of "
             "hashrate. Top 4 Ethereum miners (pre-Merge) controlled "
             "61%."),
            ("Geographic concentration.",
             "Roughly 30% of Bitcoin nodes live in a single "
             "autonomous system. Over half in one country "
             "(pre-China-ban)."),
            ("Nakamoto coefficient.",
             "Kwon 2017: fewer than 10 entities could collude to "
             "halt Bitcoin or Ethereum."),
            ("The 'decentralized' framing is aspirational, not "
             "empirical.",
             "Real PoW and PoS networks have concentration "
             "comparable to banking oligopolies."),
        ])
    add_footer(s, 28, TOTAL, BRAND)

    s = _im(prs, "Finality latency: log scale",
        "chart_finality.png",
        caption="Sources: official docs, published benchmarks, "
                "Ethereum Foundation, Algorand Foundation.",
        image_h=4.2)
    add_notes(s, [
        "Bitcoin's hour-long finality horizon is unusable for "
        "point-of-sale or anything user-facing. Ethereum's 12 to 15 "
        "minute economic finality is nearly as bad.",
        "BFT systems are all sub-5-second. PoT is block-interval-"
        "bounded and deterministic per signed transaction."
    ])
    add_footer(s, 29, TOTAL, BRAND)

    s = _im(prs, "Throughput: orders of magnitude apart",
        "chart_throughput.png",
        caption="Workload matters: these are simple signed-transaction "
                "numbers. Smart contract calls cost 10 to 100x more.",
        image_h=4.2)
    add_notes(s, [
        "TPS comparisons are notoriously misleading. The chart shows "
        "best-case numbers for simple transactions, which is "
        "appropriate for a non-financial trust layer.",
        "Solana's 65k figure is claimed peak under ideal "
        "conditions; sustained production is much lower."
    ])
    add_footer(s, 30, TOTAL, BRAND)

    s = content_slide(prs,
        "Shortcoming 3: the global-state assumption", bullets=[
            ("Every mainstream consensus mechanism assumes all "
             "participants agree on a single global state.",
             ""),
            ("Appropriate for money.",
             "Double-spend prevention is the whole point. Everyone "
             "must see the same balances."),
            ("Inappropriate for trust.",
             "A reviewer with 50k positive endorsements is not "
             "equally trustworthy to every observer. Collapsing this "
             "to one number loses the signal."),
            ("Two failure modes in practice.",
             "(1) Reduce trust to a single public number, losing "
             "nuance. (2) Keep it off-chain anyway, defeating the "
             "blockchain."),
            ("'Enterprise blockchain' projects hit both of these.",
             "Hyperledger Fabric channels, Ethereum L2s, permissioned "
             "blockchain pilots are mostly attempts to paper over this "
             "mismatch."),
        ])
    add_footer(s, 31, TOTAL, BRAND)

    s = _im(prs, "MEV: the dirty secret of global consensus",
        "chart_mev.png",
        caption="Source: Flashbots MEV-Explore. MEV is the value "
                "validators extract from users via transaction "
                "reordering.",
        image_h=4.0)
    add_notes(s, [
        "Daian et al. 2020 ('Flash Boys 2.0,' IEEE S&P) documented "
        "systematic value extraction from users by Ethereum "
        "validators who reorder transactions.",
        "MEV exists because global consensus gives block producers "
        "unilateral control over ordering. Non-financial systems do "
        "not have this problem because order does not matter."
    ])
    add_footer(s, 32, TOTAL, BRAND)

    s = quote_slide(prs,
        "If your application is not money, MEV is a pure tax with "
        "no offsetting benefit. Picking an architecture immune to "
        "MEV is strictly preferable.",
        "Corollary of choosing PoT over blockchain",
        title="The MEV corollary")
    add_footer(s, 33, TOTAL, BRAND)

    # ==== SECTION 4: Proof of Trust (34-41) ====
    s = section_divider(prs, 4, "Proof of Trust",
        "Relational, signed, tiered, domain-scoped. Four principles.")
    add_footer(s, 34, TOTAL, BRAND)

    s = content_slide(prs, "Principle 1: relational, not global",
        bullets=[
            ("Every trust query names an observer and a target.",
             "trust(observer, target) returns a value in [0, 1]."),
            ("Same target, different observer, different answer.",
             "Alice sees Carol at 0.63. Dave sees Carol at 0.28. "
             "Both are correct; both derive from the same chain."),
            ("This is the opposite of global consensus.",
             "We deliberately allow observers to maintain distinct "
             "views. It is a feature, not a bug."),
            ("Formally.",
             "RT(u, v) = max { product-of-weights along any simple "
             "path from u to v } over the trust graph."),
            ("The graph is append-only.",
             "Edges are signed TRUST transactions. Revocation is a "
             "later signed TRUST with weight zero."),
        ])
    add_footer(s, 35, TOTAL, BRAND)

    s = _im(prs, "Alice and Dave see Carol differently, and that's correct",
        "chart_divergent_views.png",
        caption="Same chain state. Same edges. Different trust graph "
                "positions for the two observers.",
        image_h=4.4)
    add_notes(s, [
        "This is the canonical example. Walk through the arithmetic: "
        "Alice trusts Bob at 0.9, Bob trusts Carol at 0.7, so Alice "
        "sees Carol at 0.9 times 0.7 = 0.63.",
        "Dave trusts Bob at 0.4, so Dave sees Carol at 0.4 times 0.7 "
        "= 0.28. Both views are correct. Forcing one of them on the "
        "other is data loss."
    ])
    add_footer(s, 36, TOTAL, BRAND)

    s = content_slide(prs, "Principle 2: signed, not raced", bullets=[
        ("Every transaction is signed with ECDSA P-256.",
         "Signing produces a 64-byte IEEE-1363 signature. Verifying "
         "is O(1) and takes about 50 microseconds."),
        ("Signature verification is deterministic and cheap.",
         "No probabilistic finality. No race between miners. A signed "
         "transaction is valid at the moment it is issued."),
        ("What the chain does.",
         "Orders signed transactions into blocks for replay + "
         "durability. Not for validity."),
        ("Compare to PoW.",
         "In PoW, validity depends on hashrate winning the race. In "
         "PoT, validity depends on the signer's key being known."),
        ("Corollary.",
         "The 'finality vs liveness' tradeoff disappears for "
         "individual transactions. Finality is instant."),
    ])
    add_footer(s, 37, TOTAL, BRAND)

    s = _im(prs, "Principle 3: tiered block acceptance",
        "chart_tiered.png",
        caption="Each observer classifies blocks by their local "
                "trust in the producing validator.",
        image_h=4.4)
    add_notes(s, [
        "Instead of binary accept/reject (as in PoW/PoS), PoT "
        "classifies each block into Trusted / Tentative / Rejected "
        "based on the observer's trust in the producing validator.",
        "This is per-observer. Two observers can legitimately "
        "disagree about the same block's tier.",
        "Trusted blocks integrate into the main chain view; "
        "Tentative blocks are held for reconsideration; Rejected "
        "blocks are dropped."
    ])
    add_footer(s, 38, TOTAL, BRAND)

    s = _im(prs, "Principle 4: domain scoping",
        "chart_domain_scope.png",
        caption="Trust is not globally transitive. "
                "hospital.records does not imply finance.*",
        image_h=4.4)
    add_notes(s, [
        "Domain scoping is what PGP Web of Trust missed and PoT "
        "gets right by construction.",
        "Alice trusting Bob at 0.9 in hospital.records says nothing "
        "about finance.consulting or identity.persona. Each domain "
        "has its own trust graph."
    ])
    add_footer(s, 39, TOTAL, BRAND)

    s = content_slide(prs, "The whole picture", bullets=[
        ("No leader election.",
         "Any consortium member can produce a block at the designated "
         "interval."),
        ("No voting rounds.",
         "Validity is cryptographic; acceptance is per-observer."),
        ("No probabilistic finality.",
         "Signed transactions are final at signature time."),
        ("No economic staking.",
         "Sybil resistance is structural (trust graph), not economic."),
        ("No global state agreement.",
         "Observers can diverge indefinitely on tentative blocks."),
        ("Just: signed transactions + a trust graph.",
         "That is the entire protocol surface."),
    ])
    add_footer(s, 40, TOTAL, BRAND)

    s = content_slide(prs, "Inheritance and relation to FBA", bullets=[
        ("Stellar's FBA (Mazieres 2015) is the closest conceptual ancestor.",
         "SCP formalized observer-chosen quorum slices. Quidnug goes "
         "further: graded trust weights, not binary slices."),
        ("Differences from FBA.",
         "Scope: PoT adds domain scoping; SCP is single-domain."),
        ("Decay: PoT has multiplicative path decay; SCP has binary "
         "membership.",
         ""),
        ("Divergence: PoT allows permanent observer divergence; SCP "
         "expects eventual convergence via slice intersection.",
         ""),
        ("Revocation: PoT has time-bounded edges with TTL; SCP has "
         "out-of-band slice updates.",
         ""),
    ])
    add_footer(s, 41, TOTAL, BRAND)

    # ==== SECTION 5: Math (42-51) ====
    s = section_divider(prs, 5, "The Math of Relational Trust",
        "Four properties, with proofs.")
    add_footer(s, 42, TOTAL, BRAND)

    s = _im(prs, "Trust propagates multiplicatively along paths",
        "chart_trust_decay.png",
        caption="T(P) = product of edge weights along the path. "
                "All weights in [0,1], so the product decays "
                "monotonically.",
        image_h=4.2)
    add_footer(s, 43, TOTAL, BRAND)

    s = code_slide(prs, "Definitions",
        [
            '// Trust graph: G = (V, E, w)',
            '//   V = set of quids',
            '//   E ⊆ V × V = directed trust edges',
            '//   w: E → [0, 1] = edge weight',
            '',
            '// Path trust: for a simple path P = v_0 → v_1 → ... → v_k',
            'T(P) = w(v_0, v_1) · w(v_1, v_2) · ... · w(v_{k-1}, v_k)',
            '',
            '// Relational trust: best-path aggregation',
            'RT(observer, target) = max { T(P) : P is a simple path',
            '                              from observer to target }',
            '',
            '// Boundary cases:',
            'RT(u, u)   = 1     // full self-trust',
            'RT(u, v)   = 0     // if no path exists',
        ])
    add_footer(s, 44, TOTAL, BRAND)

    s = content_slide(prs,
        "Property 1: monotonic decay (proof)", bullets=[
            ("Claim.",
             "For any path P of length k >= 1 with edge weights in "
             "[0, 1], T(P) <= min of the edge weights on P."),
            ("Proof sketch.",
             "Each edge weight is at most 1. Multiplying by any "
             "value <= 1 cannot increase the product. Formally: "
             "T(P) = w_j · product of others <= w_j · 1^(k-1) = w_j."),
            ("Therefore T(P) <= min_j w_j.",
             ""),
            ("Consequence.",
             "A single weak edge caps the entire path's trust. "
             "Attackers with only low-weight connections cannot "
             "produce high-trust outputs, regardless of chain length."),
            ("Contrast with additive models.",
             "In an additive scheme, many weak edges could sum to "
             "a high score. Multiplicative decay prevents this "
             "structurally."),
        ])
    add_footer(s, 45, TOTAL, BRAND)

    s = content_slide(prs,
        "Property 2: structural Sybil resistance", bullets=[
            ("Claim.",
             "For an observer u with total out-edge weight budget B, "
             "the aggregate trust u can confer on a Sybil cluster is "
             "bounded by B."),
            ("Proof sketch.",
             "Suppose u issues trust to Sybils s_1, ..., s_k with "
             "weights w_1, ..., w_k summing to at most B. "
             "Each Sybil can derive trust at most w_i in the direct "
             "case, and through chains, no more than max w_i × "
             "(decayed factor <= 1)."),
            ("Key implication.",
             "Flooding the chain with Sybils does not help an "
             "attacker unless the observer trusts at least one of "
             "them directly."),
            ("This is why PoT does not need PoW-style or PoS-style "
             "Sybil resistance.",
             "The resistance is structural: Sybils that nobody "
             "trusts are invisible in the trust computation."),
        ])
    add_footer(s, 46, TOTAL, BRAND)

    s = _im(prs, "Cost-of-attack asymmetry",
        "chart_sybil.png",
        caption="Creating Sybils is cheap everywhere. Moving an "
                "observer's view is the relevant cost.",
        image_h=4.2)
    add_notes(s, [
        "In OAuth / global reputation systems, the attacker creates "
        "fake accounts and floods reviews. Quantity is the attack.",
        "In PoT, Sybils that no observer trusts do not affect any "
        "observer's computation. Quantity does not help. The "
        "binding cost is getting trusted by the target observer, "
        "which does not scale."
    ])
    add_footer(s, 47, TOTAL, BRAND)

    s = content_slide(prs,
        "Property 3: bounded computation", bullets=[
            ("Claim.",
             "Computing RT(u, v) with maximum depth d has worst-case "
             "time complexity O(b^d), where b is the average "
             "out-degree of the graph."),
            ("Proof.",
             "BFS from u to depth d visits at most 1 + b + b^2 + ... "
             "+ b^d nodes. Each edge traversal is O(1). Total: "
             "O(b^d)."),
            ("Typical numbers.",
             "Quidnug defaults to d = 5. For b ~ 10, that is ~100k "
             "node visits worst case."),
            ("With memoization and TTL filtering.",
             "Real queries resolve in under 1 millisecond."),
            ("Compare to blockchain-based trust queries.",
             "On-chain reputation typically requires full-node state "
             "access, which is 100 to 1000x slower."),
        ])
    add_footer(s, 48, TOTAL, BRAND)

    s = content_slide(prs,
        "Property 4: convergence under honest-majority gossip",
        bullets=[
            ("Claim.",
             "Under partial synchrony, if over 50% of consortium "
             "validators are honest and gossip is reliable, all "
             "honest observers converge on the same Trusted-tier "
             "block set within O(gossip_delay × diameter) time."),
            ("Proof sketch.",
             "Honest validators produce blocks that propagate through "
             "gossip to all observers within bounded delay. Trust "
             "edges to the validator set also propagate. Observers' "
             "views converge as gossip completes."),
            ("Divergence persists only for observers who maintain "
             "different trust edges toward validators.",
             "That is a feature of the relativistic model."),
            ("Practical impact.",
             "Observers who trust the same consortium see the same "
             "chain. Observers who trust different consortia see "
             "different chains. Both are correct."),
        ])
    add_footer(s, 49, TOTAL, BRAND)

    s = table_slide(prs,
        "Properties summary: PoT against three major alternatives",
        [
            ["Property", "PoW", "BFT", "PoT"],
            ["Finality type",
             "Probabilistic",
             "Deterministic",
             "Deterministic per signed tx"],
            ["Sybil resistance",
             "Economic (hashrate)",
             "N/A (known set)",
             "Structural (trust graph)"],
            ["Verification cost / tx",
             "O(1), block-level",
             "O(n) messages",
             "O(1) per tx + O(b^d) per trust query"],
            ["Partition response",
             "Forks, resolve via longest chain",
             "Halts",
             "Observers may diverge indefinitely"],
            ["Throughput upper bound",
             "Block size / interval",
             "Network RTT",
             "Per-signer serial + per-domain parallel"],
        ],
        col_widths=[2.0, 1.8, 1.8, 2.5], body_size=11)
    add_footer(s, 50, TOTAL, BRAND)

    # ==== SECTION 6: Security (51-57) ====
    s = section_divider(prs, 6, "Security Properties",
        "Replay, rotation, Byzantine tolerance, TTL, revocation.")
    add_footer(s, 51, TOTAL, BRAND)

    s = content_slide(prs, "Replay prevention via nonce ledger",
        bullets=[
            ("Every signed transaction carries a monotonic nonce.",
             "The NonceLedger (QDP-0001) enforces that no nonce "
             "repeats for a given (signer, domain, epoch) tuple."),
            ("Claim.",
             "Given a correct NonceLedger, no accepted transaction "
             "can be replayed."),
            ("Proof sketch.",
             "Replay of tx with nonce n fails because n <= max "
             "accepted nonce. A forged tx' with nonce n has different "
             "content, so its signature fails verification. Cross-"
             "domain submission is a separate authorized action, not "
             "a replay."),
            ("Cross-domain nonce scoping (QDP-0003).",
             "(signer, domain_A, epoch) and (signer, domain_B, epoch) "
             "are independent. A signer explicitly authorizes each."),
        ])
    add_footer(s, 52, TOTAL, BRAND)

    s = content_slide(prs, "Key rotation: bounding compromise damage",
        bullets=[
            ("QDP-0001 supports key rotation via signed NonceAnchor.",
             "After rotation, old signatures remain verifiable; new "
             "signatures use the new key."),
            ("Compromise damage window.",
             "If compromise occurs at time T_c and rotation happens "
             "at T_r, attacker damage is bounded by [T_c, T_r]."),
            ("Post-rotation, compromised key signatures fail.",
             "Because they do not match the new public key "
             "registered for that epoch."),
            ("Guardian Recovery (QDP-0002).",
             "M-of-N guardian-cosigned transaction can rotate a lost "
             "or stolen key. Recovery is a social + cryptographic "
             "event, not a manual key-management nightmare."),
            ("Compare to 'your key IS your identity.'",
             "Bitcoin's model: lose the key, lose everything. PoT's "
             "model: rotate the key, keep the identity."),
        ])
    add_footer(s, 53, TOTAL, BRAND)

    s = content_slide(prs, "Byzantine consortium tolerance",
        bullets=[
            ("Claim.",
             "Even if a minority of consortium validators are "
             "Byzantine, honest observers' views are not corrupted, "
             "provided the observer's trust graph reflects their "
             "actual trust."),
            ("Proof sketch.",
             "A Byzantine validator produces a malicious block. The "
             "observer evaluates trust in the validator; if below "
             "threshold, block is Tentative or Rejected."),
            ("Different from classical BFT.",
             "No global quorum is needed. Each observer decides. "
             "Byzantine validators can produce blocks some observers "
             "accept (those who trust them) and others reject."),
            ("System-level safety.",
             "Network-wide corruption requires corrupting enough "
             "validators to cross observers' trust thresholds. In a "
             "maintained trust graph, that is a social attack, not a "
             "cryptographic one."),
            ("This is a different guarantee, not a weaker one.",
             "Per-observer deterministic validity, not global "
             "quorum."),
        ])
    add_footer(s, 54, TOTAL, BRAND)

    s = content_slide(prs, "TTL and revocation (QDP-0022)",
        bullets=[
            ("Every trust edge carries a ValidUntil timestamp.",
             "Expired edges are automatically filtered from trust "
             "queries. No separate revocation call required."),
            ("Claim.",
             "Revocation is effective within max(gossip_delay, "
             "observer_polling_interval) of a revocation event."),
            ("Proof sketch.",
             "A revocation is a new TRUST with weight 0 or shortened "
             "ValidUntil. It propagates via the standard gossip "
             "protocol. Observers see the update on their next trust "
             "computation."),
            ("Contrast with X.509 CRLs (slow) or OCSP (per-query).",
             "PoT revocation is 'tell everyone, they all update, next "
             "query reflects it.' Memoized caches invalidate "
             "per-observer."),
            ("TTL + revocation together.",
             "TTL bounds damage from lost revocations; revocation "
             "handles explicit takedowns. Defense in depth."),
        ])
    add_footer(s, 55, TOTAL, BRAND)

    s = quote_slide(prs,
        "Structural defense beats detection. "
        "A verifier that rejects out-of-scope trust chains "
        "cryptographically does not depend on catching the attack.",
        "The design philosophy of signed delegation chains",
        title="Structural > detection")
    add_footer(s, 56, TOTAL, BRAND)

    s = _im(prs,
        "Six-metric comparison: consensus mechanisms side by side",
        "chart_comparison.png",
        caption="Author assessment based on published benchmarks and "
                "real-world observation.",
        image_h=4.0)
    add_footer(s, 57, TOTAL, BRAND)

    # ==== SECTION 7: Real-World Use Cases (58-70) ====
    s = section_divider(prs, 7, "Real-World Use Cases",
        "Where PoT is the right answer, and where it is not.")
    add_footer(s, 58, TOTAL, BRAND)

    s = content_slide(prs, "Use case 1: reviews and reputation (PoT wins)",
        bullets=[
            ("The problem.",
             "Yelp, TripAdvisor, Amazon reviews. Reviewer credibility "
             "varies. Observers care about different reviewers."),
            ("Why global consensus fails.",
             "A single public trust score per reviewer loses the "
             "relational structure. A foodie trusts different "
             "reviewers than a casual diner."),
            ("Why PoT fits.",
             "Each observer has their own trust graph. Trust in a "
             "reviewer decays multiplicatively. Observers see "
             "different aggregate ratings, and that is correct."),
            ("Implementation.",
             "Quidnug Reviews Protocol (QRP-0001) with four-factor "
             "rating: topical trust, helpfulness, activity, "
             "recency."),
            ("See the 'Relativistic Ratings' blog post and deck for "
             "full treatment.",
             ""),
        ])
    add_footer(s, 59, TOTAL, BRAND)

    s = content_slide(prs,
        "Use case 2: healthcare consent (PoT wins)", bullets=[
            ("The requirement.",
             "HIPAA, GDPR Article 9, LGPD require granular, "
             "revocable, auditable consent."),
            ("Why PoW or PoS fails.",
             "Patient identifiers on a public ledger is a regulatory "
             "catastrophe. Privacy-preserving variants add complexity "
             "and reintroduce trust in the private operators."),
            ("Why PoT fits.",
             "Signed consent records on permissioned domains like "
             "hospital.patients. Revocation is a new signed "
             "transaction; cascade revokes dependent grants."),
            ("GDPR erasure via cryptographic shredding.",
             "CID-stored payloads can be made unreachable by key "
             "deletion (QDP-0015 and 0017). Right to be forgotten "
             "without breaking the audit trail."),
            ("Compliance mapping.",
             "Attribution of every consent event. Cross-processor "
             "audit with cryptographic proof."),
        ])
    add_footer(s, 60, TOTAL, BRAND)

    s = content_slide(prs,
        "Use case 3: interbank wire authorization (PoT wins)",
        bullets=[
            ("SWIFT processes ~45M messages/day.",
             "TARGET2 and Fedwire settlement finality is seconds. "
             "Existing rails work operationally."),
            ("Blockchain-based interbank pilots.",
             "JPMorgan Onyx, Fnality USC, R3 Corda converged on "
             "permissioned designs because PoW/PoS tradeoffs made no "
             "sense for this use case."),
            ("Why PoT fits.",
             "Banks have well-defined bilateral trust. "
             "ISO 20022 messages are signed end-to-end. PoT with "
             "domain per corridor (wires.usd-eur.*) provides "
             "attribution without mining."),
            ("Reference implementation.",
             "Quidnug's UseCases/interbank-wire-authorization/ runs "
             "sub-100ms end-to-end at zero energy overhead."),
            ("Extends naturally to FX, clearing, securities.",
             "Same trust model, different domain tree."),
        ])
    add_footer(s, 61, TOTAL, BRAND)

    s = content_slide(prs,
        "Use case 4: elections with ballot anonymity (PoT + blind sigs)",
        bullets=[
            ("The two hard requirements.",
             "Universal verifiability: anyone can check the tally. "
             "Ballot secrecy: nobody can link a ballot to a voter."),
            ("Why blockchain voting pilots failed.",
             "West Virginia Voatz (2020), Swiss Post SwissVote (2019) "
             "both had security holes around the auth/issuance "
             "separation."),
            ("PoT + blind signatures.",
             "Eligibility is signed via identity domain. Ballot "
             "issuance uses RSA-FDH blind signatures (QDP-0021). "
             "Voter anonymity + cryptographic verifiability."),
            ("Implementation.",
             "pkg/crypto/blindrsa in the Quidnug reference repo. "
             "Landed 2025."),
            ("Audit path.",
             "Anyone with the election authority's public keys can "
             "verify the tally. No individual vote is revealed."),
        ])
    add_footer(s, 62, TOTAL, BRAND)

    s = content_slide(prs,
        "Use case 5: supply chain provenance (PoT wins)", bullets=[
            ("The supply chain trust problem.",
             "Producers sign attestations. Auditors endorse "
             "producers. Buyers maintain their own trust graph."),
            ("Why IBM TradeLens failed.",
             "Forced global consensus when buyers legitimately have "
             "different views of which producers and auditors are "
             "trustworthy."),
            ("Why PoT fits.",
             "Product provenance = chain of signed attestations. "
             "Trustworthiness to a specific buyer = path in that "
             "buyer's trust graph."),
            ("Regional adaptation.",
             "US buyer trusts FDA and USDA. EU buyer trusts EFSA. "
             "Same provenance chain, different trust-graph weights."),
            ("Integrations.",
             "C2PA for media, Sigstore for software artifacts. Trust "
             "layer composes with existing verification standards."),
        ])
    add_footer(s, 63, TOTAL, BRAND)

    s = content_slide(prs,
        "Use case 6: identity as a trust-graph composite (DID + PoT)",
        bullets=[
            ("W3C Decentralized Identifiers (DIDs) give you an "
             "identifier format and resolution standard.",
             ""),
            ("They do not give you a trust graph.",
             "A DID resolves to a DID Document but tells you nothing "
             "about whether to trust that DID's claims."),
            ("PoT complements DIDs.",
             "did:quidnug:c7e2d10000000001 resolves to a public key "
             "in the Quidnug identity registry. Trust graph wraps it "
             "with per-observer weights."),
            ("QDP-0023 DNS-anchored attestation.",
             "Binds DIDs to existing DNS trust roots for "
             "bootstrapping."),
            ("Cross-platform portable identity.",
             "Your DID resolves the same way everywhere. Your trust "
             "graph is per-observer."),
        ])
    add_footer(s, 64, TOTAL, BRAND)

    s = content_slide(prs, "Use case 7: where PoT is the wrong tool",
        bullets=[
            ("Permissionless tokenized money.",
             "Observer divergence is unacceptable for double-spend "
             "prevention. Use Bitcoin or an L2."),
            ("Turing-complete smart contracts with global state.",
             "State must be identical for all observers. Use Ethereum "
             "PoS."),
            ("High-frequency trading on public order books.",
             "Sub-millisecond latency and MEV-resistant ordering. "
             "Use a centralized matching engine + on-chain "
             "settlement."),
            ("Adversarial voting with no authority.",
             "PoT assumes an authority can issue blind-signed "
             "ballots. For fully authority-less voting, use ZK-SNARKs "
             "(Helios, MACI)."),
            ("Decentralized storage consensus (Filecoin).",
             "Requires economic commitments tied to storage. Use "
             "PoSt / PoRep."),
        ])
    add_footer(s, 65, TOTAL, BRAND)

    s = icon_grid_slide(prs, "PoT use case fit matrix",
        [
            ("Reviews / reputation", "Per-observer is the whole point. "
             "Huge win.", EMERALD),
            ("Healthcare consent", "Granular, revocable, audited. "
             "Clean fit.", EMERALD),
            ("Supply chain provenance", "Bilateral trust, cross-"
             "border audit. Native fit.", EMERALD),
            ("Interbank wires", "Bilateral trust, ISO 20022, sub-"
             "second. Natural fit.", EMERALD),
            ("Elections (with blind sigs)", "Verifiability + "
             "anonymity. Good fit with companion primitive.", EMERALD),
            ("Identity (with DID)", "Complementary to W3C DID. "
             "Natural layer.", EMERALD),
            ("Permissionless money",
             "Observer divergence not OK. Wrong tool.", CORAL),
            ("Turing-complete contracts",
             "Needs global state. Wrong tool.", CORAL),
            ("High-frequency trading",
             "Sub-ms latency required. Wrong tool.", CORAL),
        ],
        cols=3, subtitle="Green = PoT shines; red = use something else.")
    add_footer(s, 66, TOTAL, BRAND)

    # ==== SECTION 8: Benchmarks + Tradeoffs (67-77) ====
    s = section_divider(prs, 8, "Benchmarks and Tradeoffs",
        "Hard numbers on throughput, latency, energy. Honest about limits.")
    add_footer(s, 67, TOTAL, BRAND)

    s = content_slide(prs, "Benchmark caveat before the numbers",
        bullets=[
            ("TPS comparisons are notoriously misleading.",
             "A signature-only transaction is cheap. A smart-contract "
             "call can cost 100x more."),
            ("Latency depends on network topology.",
             "BFT protocols shine on low-latency LANs and suffer "
             "across continents."),
            ("Energy depends on how you measure.",
             "Just the validators? The whole network? The "
             "manufacturing footprint of ASICs?"),
            ("All numbers below assume simple signed transactions.",
             "Apples to apples, for the workload PoT is designed "
             "for."),
            ("Worst-case numbers for Quidnug come from its own "
             "tests/benchmarks/ directory.",
             "Best-case numbers for competing systems come from "
             "their marketing. The comparison is generous to "
             "competitors."),
        ])
    add_footer(s, 68, TOTAL, BRAND)

    s = content_slide(prs, "Throughput: where PoT fits", bullets=[
        ("Bitcoin PoW: 7 tx/s.",
         "This is not a benchmark; it is a protocol limit."),
        ("Ethereum L1: 15 tx/s. L2s push it higher at cost of "
         "complexity.",
         ""),
        ("BFT family: 1,000 to 1,500 tx/s.",
         "Tendermint, HotStuff in production."),
        ("DAG family: 10,000+ tx/s.",
         "Hedera Hashgraph, Avalanche."),
        ("Quidnug PoT per domain: ~5,000 tx/s measured.",
         "Scales horizontally via domain partitioning."),
        ("Per-domain is the right unit.",
         "Healthcare transactions do not compete with interbank wires "
         "for throughput in the same physical chain."),
    ])
    add_footer(s, 69, TOTAL, BRAND)

    s = content_slide(prs, "Finality: where PoT shines",
        bullets=[
            ("Bitcoin PoW: 1 hour (6 confirmations).",
             "Probabilistic, not deterministic."),
            ("Ethereum PoS: 12.8 minutes (2 epochs).",
             "Economic finality, can revert under specific "
             "attacker scenarios."),
            ("Tendermint, HotStuff: 1 to 3 seconds.",
             "Deterministic BFT finality."),
            ("Quidnug PoT: block interval (default 60s, "
             "configurable).",
             "Deterministic per signed transaction; block interval "
             "is for ordering."),
            ("Critical distinction.",
             "Quidnug individual transactions are final at signature "
             "time. Blocks provide ordering; they do not provide "
             "validity."),
        ])
    add_footer(s, 70, TOTAL, BRAND)

    s = content_slide(prs, "Energy: where PoT wins outright",
        bullets=[
            ("Bitcoin PoW: ~10 MJ per transaction.",
             "Literally burning a gallon of gasoline for every wire "
             "transfer equivalent."),
            ("Ethereum PoS post-merge: ~0.2 J per transaction.",
             "Genuinely low."),
            ("BFT: ~0.004 J per transaction.",
             "Tendermint, HotStuff. Dominated by network I/O."),
            ("Quidnug PoT: ~0.005 J per transaction.",
             "Commodity hardware, ECDSA signature verify, gossip. "
             "Effectively in the noise."),
            ("Real-world context.",
             "Sending a WhatsApp message costs more energy than a "
             "Quidnug trust-query. Bitcoin tx costs more than a "
             "cross-Atlantic flight per passenger, per tx."),
        ])
    add_footer(s, 71, TOTAL, BRAND)

    s = _im(prs, "Decision tree: which consensus mechanism?",
        "chart_decision.png",
        caption="Start top-left. Follow the branches. Most real-world "
                "applications land on Kafka or PoT.",
        image_h=5.0)
    add_footer(s, 72, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 1: no bootstrap from strangers",
        bullets=[
            ("PoW and PoS allow strangers to contribute immediately.",
             "Run a miner or validator, earn a stake."),
            ("PoT requires someone to trust you first.",
             "A brand-new quid with zero trust edges is invisible."),
            ("This is a feature for most applications.",
             "No anonymous Sybil flooding."),
            ("It is a hard blocker for some applications.",
             "Permissionless anonymous participation cannot be "
             "provided."),
            ("Mitigations.",
             "OIDC bridge + operator-attested identity; DNS-anchored "
             "attestation (QDP-0023); reputation graduation over "
             "time via QDP-0016."),
        ])
    add_footer(s, 73, TOTAL, BRAND)

    s = content_slide(prs,
        "Honest tradeoff 2: observer divergence isn't always a feature",
        bullets=[
            ("Some applications require global agreement.",
             "A reviewer suspended for fraud should probably be "
             "un-trustworthy to every observer."),
            ("PoT allows divergence.",
             "Observers who have not received the revocation see the "
             "old state."),
            ("Mitigations.",
             "QDP-0015 moderation (structural suppression), QDP-0018 "
             "audit logs, federation-wide gossip of consensus signals."),
            ("Social convention, not protocol guarantee.",
             "The operator who claims to honor moderation is "
             "incentivized to maintain consistency with peers."),
            ("This is the same problem as real-world trust.",
             "Reputation really is subjective. PoT acknowledges that; "
             "global consensus pretends it is not."),
        ])
    add_footer(s, 74, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 3: no smart contracts",
        bullets=[
            ("Ethereum's EVM is a general computational substrate.",
             "Quidnug does not try to replicate it."),
            ("If you need turing-complete on-chain logic.",
             "Use Ethereum or a similar smart-contract platform."),
            ("PoT handles signed messages with structured payloads.",
             "For most non-money applications, that is what the "
             "application actually needs."),
            ("Integration path.",
             "Quidnug's Chainlink External Adapter lets on-chain "
             "contracts query Quidnug trust scores. Best of both "
             "worlds where both are needed."),
            ("Dogma-free recommendation.",
             "Use the right tool. EVM for Uniswap. Quidnug for consent "
             "management. There is no free lunch and no single "
             "substrate fits all."),
        ])
    add_footer(s, 75, TOTAL, BRAND)

    s = content_slide(prs, "Honest tradeoff 4: mathematical maturity",
        bullets=[
            ("PoW has been formally analyzed for 17 years.",
             "PBFT for 25 years. HotStuff, Algorand, Ouroboros have "
             "peer-reviewed security proofs."),
            ("PoT as Quidnug implements is newer.",
             "Primitives are mature (signatures, trust graphs, "
             "append-only logs). The specific combination has less "
             "formal scrutiny."),
            ("Ongoing formal work.",
             "Relational trust's correctness properties are under "
             "external formal review. Peer-reviewed writeup targeted "
             "for 2026-Q4."),
            ("Caution on high-stakes financial applications.",
             "For money movement, wait for formal verification. "
             "For reputation, consent, supply chain: the primitives "
             "are mature enough today."),
            ("Review and critique welcome.",
             "github.com/quidnug/quidnug Discussions tab or any peer-"
             "reviewed venue."),
        ])
    add_footer(s, 76, TOTAL, BRAND)

    s = quote_slide(prs,
        "If your application does not need every observer to agree "
        "on the same state, you do not need a blockchain. You need "
        "signed data, a trust graph, and a protocol that composes "
        "them.",
        "The takeaway in one sentence",
        title="The takeaway in one sentence")
    add_footer(s, 77, TOTAL, BRAND)

    # ==== SECTION 9: Closing (78-90) ====
    s = section_divider(prs, 9, "Decision Framework + Closing",
        "Summary, takeaways, questions.")
    add_footer(s, 78, TOTAL, BRAND)

    s = content_slide(prs, "A three-question decision framework",
        bullets=[
            ("Question 1. Is the primary value tokenized?",
             "If yes (money, NFTs, staking), you likely need global "
             "consensus. PoW or PoS."),
            ("Question 2. Do all observers need identical state?",
             "If yes (settlement finality, regulatory uniform "
             "reporting), you need global consensus. BFT or PoS."),
            ("Question 3. Is this fundamentally about trust?",
             "If yes (reviews, consent, provenance, identity, "
             "attestations), PoT is the natural fit."),
            ("Most real-world systems.",
             "Answer no/no/yes. The default choice should be PoT, not "
             "a blockchain."),
            ("Honest recommendation.",
             "Do not pick a substrate before answering these three "
             "questions."),
        ])
    add_footer(s, 79, TOTAL, BRAND)

    s = icon_grid_slide(prs, "Recommendations by use case",
        [
            ("Global money", "Bitcoin, or Ethereum L2s.", CORAL),
            ("Smart contracts", "Ethereum, Solana, Sui.", AMBER),
            ("Permissioned DeFi", "Tendermint, HotStuff.", AMBER),
            ("Enterprise data sharing", "Kafka or streaming DB.", TEXT_MUTED),
            ("Review systems", "PoT (Quidnug).", TEAL),
            ("Consent management", "PoT (Quidnug).", TEAL),
            ("Supply chain", "PoT + Sigstore / C2PA.", TEAL),
            ("Identity attestation", "DIDs + PoT.", TEAL),
            ("Elections", "PoT + blind signatures.", TEAL),
        ], cols=3)
    add_footer(s, 80, TOTAL, BRAND)

    s = content_slide(prs, "What this talk argued",
        bullets=[
            ("1. Global consensus solves a narrow problem (money).",
             "It was never meant as a universal distributed-systems "
             "primitive."),
            ("2. FLP + CAP say any design is a choice.",
             "Nakamoto chose availability. BFT chose consistency. "
             "PoT chose per-observer truth."),
            ("3. Relational trust has clean mathematical properties.",
             "Monotonic decay, structural Sybil resistance, bounded "
             "computation, convergence under honest majority."),
            ("4. The cost of PoT is low.",
             "200 microseconds per trust query, zero energy above "
             "commodity compute."),
            ("5. The benefit is large.",
             "Most real-world trust problems are relational, not "
             "global. The substrate should match."),
        ])
    add_footer(s, 81, TOTAL, BRAND)

    s = content_slide(prs, "What to do Monday morning",
        bullets=[
            ("Audit your application's identity and trust assumptions.",
             "Is there a single 'trust score' for each entity? Why?"),
            ("Read the Greshake 2023 and Ioannidis 2005 papers.",
             "They set up the indirect-injection and reproducibility "
             "failure modes that motivate rethinking this layer."),
            ("Prototype a Quidnug consortium for one domain.",
             "Reviews, consent, supply-chain attestation: pick the "
             "one that fits your application's pain point."),
            ("Benchmark.",
             "Quidnug ships with tests/benchmarks/ and reference "
             "nodes in deploy/. Stand up a 3-node local cluster in "
             "an afternoon."),
            ("Contribute back if it works.",
             "github.com/quidnug/quidnug accepts QDPs via PR. Every "
             "added use case benefits the ecosystem."),
        ])
    add_footer(s, 82, TOTAL, BRAND)

    s = content_slide(prs, "What this protocol does not solve",
        bullets=[
            ("Money movement between untrusting strangers.",
             "That remains PoW's job. Bitcoin is fine."),
            ("Compromised root operators.",
             "If the seed-operator key is hostile, cryptographic "
             "defense cannot compensate. Social trust is the root."),
            ("Social engineering on humans.",
             "If Alice signs a malicious trust edge, PoT dutifully "
             "records her intent. Garbage in, garbage out."),
            ("Quantum cryptographic compromise.",
             "ECDSA P-256 will not survive a large quantum computer. "
             "Migration via QDP-0020 protocol versioning is "
             "planned."),
            ("Fundamental physical impossibilities.",
             "CAP is still a theorem. FLP is still a theorem. PoT is "
             "a choice within those constraints, not a magical "
             "bypass."),
        ])
    add_footer(s, 83, TOTAL, BRAND)

    s = content_slide(prs, "What comes next for Quidnug", bullets=[
        ("Phases 2-6 of existing QDPs.",
         "Moderation CLI, DSR auto-fulfill, PoW challenges, "
         "governance transactions, AUDIT_ANCHOR events, reputation "
         "graduation."),
        ("Formal verification.",
         "Peer-reviewed proofs of the relational-trust properties "
         "we walked in Section 5. Target: late 2026."),
        ("Client SDK maturity.",
         "Python and Go are battle-tested. TypeScript, Rust, Java, "
         ".NET coming up to parity."),
        ("Integrations.",
         "Chainlink, Sigstore, C2PA, HL7 FHIR, ISO 20022, Schema.org "
         "are shipped. Next: Kafka, OpenTelemetry."),
        ("Network effects.",
         "Each use case adoption strengthens the shared trust "
         "graph. Reviews help interbank wires. Attestations help "
         "consent."),
    ])
    add_footer(s, 84, TOTAL, BRAND)

    s = content_slide(prs, "How to get involved",
        bullets=[
            ("Read the blog post this deck is based on.",
             "blogs/2026-04-20-proof-of-trust-vs-consensus-"
             "mechanisms.md at the Quidnug repo."),
            ("Run a local node.",
             "docker-compose up in deploy/docker/ gives you a 3-node "
             "cluster with IPFS and Prometheus in 5 minutes."),
            ("Propose a QDP.",
             "The protocol is extensible. New primitives land via "
             "the QDP process in docs/design/."),
            ("Integrate with your stack.",
             "Client SDKs in Python, Go, TS, Rust. Docs at "
             "quidnug.com/docs."),
            ("Critique the math.",
             "Section 5's proofs are in the repo. Find a flaw, open "
             "an issue, get credited."),
        ])
    add_footer(s, 85, TOTAL, BRAND)

    s = content_slide(prs, "Common objections, briefly answered",
        bullets=[
            ("'Sounds like PGP Web of Trust, which failed.'",
             "Correct lineage. PoT fixes PGP's three failure modes: "
             "no decay, no scoping, no revocation."),
            ("'Without global consensus, I can't trust the state.'",
             "You can; you trust the state to the degree you trust "
             "the validators. That is what real trust looks like."),
            ("'This sounds slow.'",
             "200 microseconds per query on a laptop. You are already "
             "using slower infrastructure."),
            ("'This sounds complex.'",
             "ECDSA signatures + a graph walk. Less complex than EVM "
             "or PBFT. Try the reference implementation."),
            ("'It cannot replace Bitcoin.'",
             "Correct. We do not want it to. Bitcoin is fine for "
             "Bitcoin's use case."),
        ])
    add_footer(s, 86, TOTAL, BRAND)

    s = quote_slide(prs,
        "The default assumption that every distributed system needs "
        "global consensus is wrong.",
        "If you remember one line from this deck",
        title="One-line summary")
    add_footer(s, 87, TOTAL, BRAND)

    s = content_slide(prs, "References and sources",
        bullets=[
            ("Lamport, Shostak, Pease (1982). Byzantine Generals.",
             "ACM TOPLAS 4(3)."),
            ("Fischer, Lynch, Paterson (1985). Impossibility of "
             "Async Consensus.",
             "JACM 32(2)."),
            ("Castro and Liskov (1999). Practical Byzantine Fault "
             "Tolerance.",
             "OSDI '99."),
            ("Nakamoto (2008). Bitcoin: A Peer-to-Peer Electronic "
             "Cash System.",
             "bitcoin.org/bitcoin.pdf."),
            ("Eyal, Sirer (2014). Majority is Not Enough: Bitcoin "
             "Mining is Vulnerable.",
             "FC 2014."),
            ("Gencer et al. (2018). Decentralization in Bitcoin and "
             "Ethereum Networks.",
             "FC 2018."),
            ("Daian et al. (2020). Flash Boys 2.0.",
             "IEEE S&P 2020."),
            ("Cambridge CBECI. Bitcoin Electricity Consumption Index.",
             "ccaf.io/cbeci."),
        ])
    add_footer(s, 88, TOTAL, BRAND)

    s = content_slide(prs, "More references",
        bullets=[
            ("Mazieres (2015). Stellar Consensus Protocol.",
             "stellar.org/papers."),
            ("Yin et al. (2019). HotStuff: BFT Consensus with "
             "Linearity.",
             "PODC 2019."),
            ("Kiayias et al. (2017). Ouroboros.",
             "CRYPTO 2017."),
            ("Gilad et al. (2017). Algorand.",
             "SOSP 2017."),
            ("Rocket et al. (2018). Avalanche.",
             "arxiv.org/abs/1906.08936."),
            ("Popov (2018). The Tangle.",
             "iota.org whitepaper."),
            ("Quidnug QDPs 0001 through 0024.",
             "github.com/quidnug/quidnug/tree/main/docs/design."),
            ("Companion blog post.",
             "blogs/2026-04-20-proof-of-trust-vs-consensus-"
             "mechanisms.md."),
        ])
    add_footer(s, 89, TOTAL, BRAND)

    s = closing_slide(prs,
        "Questions",
        subtitle="Thank you. Now the useful part.",
        cta="Where does the PoT argument fail in your environment?\n\n"
            "Which use case did I underweight or miss?\n\n"
            "What would make you adopt this in production?",
        resources=[
            "github.com/quidnug/quidnug",
            "blogs/2026-04-20-proof-of-trust-vs-consensus-mechanisms.md",
            "Lamport et al. (1982) Byzantine Generals Problem",
            "Nakamoto (2008) Bitcoin whitepaper",
            "Mazieres (2015) Stellar Consensus Protocol",
            "Cambridge CBECI energy data: ccaf.io/cbeci",
            "Flashbots MEV data: explore.flashbots.net",
        ])
    add_footer(s, 90, TOTAL, BRAND)

    return prs


if __name__ == "__main__":
    prs = build()
    prs.save(str(OUTPUT))
    print(f"wrote {OUTPUT} ({len(prs.slides)} slides)")
