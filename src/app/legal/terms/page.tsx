// Terms of Service page (placeholder content — must be reviewed by legal).
//
// P0-9 FIX: prior to launch, the Footer had a "Terms" link that was a
// <button> with no onClick — operators and regulators expect this to
// resolve. This page provides the minimum framework: page route exists,
// renders under the standard layout, links work. The actual legal text
// MUST be drafted by a qualified gaming-law attorney in the target
// jurisdiction (Curacao / MGA / UKGC / India state-by-state).

export default function TermsPage() {
  return (
    <div className="min-h-screen bg-[#0a0a1a] text-white">
      <div className="max-w-2xl mx-auto px-4 py-12">
        <h1 className="text-2xl font-bold mb-6">Terms of Service</h1>
        <div className="prose prose-invert max-w-none">
          <p className="text-sm text-[#8892b0] mb-4">
            Last updated: {new Date().toLocaleDateString()}
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">1. Acceptance of Terms</h2>
          <p className="text-sm leading-relaxed mb-4">
            By accessing or using RockGame (&quot;the Service&quot;), you agree to be bound by these Terms of Service (&quot;Terms&quot;). If you do not agree, do not use the Service.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">2. Eligibility</h2>
          <p className="text-sm leading-relaxed mb-4">
            You must be at least 18 years old (or the legal age of majority in your jurisdiction) to use the Service. By registering, you represent and warrant that you meet this requirement.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">3. Account Registration</h2>
          <p className="text-sm leading-relaxed mb-4">
            You must provide accurate and complete information when registering. You are responsible for maintaining the confidentiality of your account credentials and for all activities under your account.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">4. Prohibited Activities</h2>
          <ul className="text-sm leading-relaxed mb-4 list-disc pl-6">
            <li>Using the Service for illegal activities or money laundering.</li>
            <li>Sharing your account credentials with others.</li>
            <li>Using bots, scripts, or automated systems to play games.</li>
            <li>Attempting to exploit bugs or vulnerabilities.</li>
            <li>Creating multiple accounts to abuse bonuses.</li>
          </ul>

          <h2 className="text-lg font-semibold mt-6 mb-3">5. Real-Money Gaming</h2>
          <p className="text-sm leading-relaxed mb-4">
            Real-money gaming is not available in all jurisdictions. You are responsible for determining whether your use of the Service is legal in your jurisdiction. The Service may require identity verification (KYC) before allowing deposits, withdrawals, or play.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">6. Payments</h2>
          <p className="text-sm leading-relaxed mb-4">
            Deposits and withdrawals are processed through third-party payment providers. Processing times and fees may vary. The Service reserves the right to delay or refuse withdrawals for fraud prevention or compliance reasons.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">7. Limitation of Liability</h2>
          <p className="text-sm leading-relaxed mb-4">
            The Service is provided &quot;as is&quot;. We are not liable for indirect, incidental, or consequential damages arising from your use of the Service.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">8. Changes to Terms</h2>
          <p className="text-sm leading-relaxed mb-4">
            We may update these Terms at any time. Continued use of the Service after changes constitutes acceptance of the new Terms.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">9. Contact</h2>
          <p className="text-sm leading-relaxed mb-4">
            For questions about these Terms, contact support@rockgame.example.
          </p>

          <p className="text-xs text-[#8892b0] mt-8 p-4 rounded bg-yellow-500/10 border border-yellow-500/30">
            ⚠️ This is a placeholder Terms of Service. Before launch, this text MUST be reviewed and finalized by a qualified gaming-law attorney in your target jurisdiction.
          </p>
        </div>
      </div>
    </div>
  );
}
