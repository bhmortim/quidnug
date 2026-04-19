import com.quidnug.client.Quid;
import com.quidnug.client.QuidnugClient;
import com.quidnug.client.QuidnugClient.IdentityParams;
import com.quidnug.client.QuidnugClient.TrustParams;

import java.util.Map;

/**
 * Enterprise-onboarding workflow — appropriate for a Java shop that
 * ingests a new vendor or counterparty.
 *
 *   1. Generate a quid for the new vendor (key material in an HSM in
 *      real production — see pkg/signer/hsm).
 *   2. Register the vendor identity with ESG / DUNS / LEI attributes.
 *   3. Authorized relationship manager grants initial trust at 0.5.
 *   4. Compliance officer grants supplementary trust at 0.3 after KYC.
 *   5. Query the relational trust from the procurement quid — vendor
 *      passes the 0.5 threshold required for the first PO.
 */
public class EnterpriseOnboarding {
    public static void main(String[] args) {
        QuidnugClient client = QuidnugClient.builder()
                .baseUrl("http://localhost:8080")
                .build();

        // Existing enterprise actors (in reality: loaded from HSM-backed
        // signer factory — see pkg/signer/hsm).
        Quid procurement = Quid.generate();
        Quid accountManager = Quid.generate();
        Quid compliance = Quid.generate();
        Quid newVendor = Quid.generate();

        // Bootstrap org: procurement trusts the account manager and
        // compliance officer at 1.0 (they're full-time employees).
        client.registerIdentity(procurement, IdentityParams.name("Acme Procurement"));
        client.registerIdentity(accountManager, IdentityParams.name("Jane Doe, AM"));
        client.registerIdentity(compliance, IdentityParams.name("Compliance Office"));
        client.grantTrust(procurement,
                TrustParams.of(accountManager.id(), 1.0, "acme.vendors"));
        client.grantTrust(procurement,
                TrustParams.of(compliance.id(), 1.0, "acme.vendors"));

        // New vendor onboarding
        client.registerIdentity(newVendor, IdentityParams
                .name("Widgets Inc.")
                .homeDomain("acme.vendors")
                .attributes(Map.of(
                        "duns", "12-345-6789",
                        "lei", "549300ABCDEF1234567890",
                        "esgRating", "BBB"
                )));

        // Account manager says: "I've worked with Widgets for 3 years"
        client.grantTrust(accountManager,
                TrustParams.of(newVendor.id(), 0.9, "acme.vendors"));

        // Compliance says: "KYC check passed, I'll vouch at 0.8"
        client.grantTrust(compliance,
                TrustParams.of(newVendor.id(), 0.8, "acme.vendors"));

        // Procurement's relational trust:
        //   procurement -> AM -> Widgets  = 1.0 * 0.9 = 0.9
        //   procurement -> Compliance -> Widgets = 1.0 * 0.8 = 0.8
        //   best path picks 0.9
        var tr = client.getTrust(procurement.id(), newVendor.id(), "acme.vendors", 5);
        System.out.printf("procurement trust in %s: %.3f via %s%n",
                newVendor.id(),
                tr.trustLevel,
                String.join(" -> ", tr.path));

        if (tr.trustLevel >= 0.7) {
            System.out.println("✓ vendor passes threshold for initial PO");
        } else {
            System.out.println("✗ vendor trust below threshold; add more attestations");
        }
    }
}
