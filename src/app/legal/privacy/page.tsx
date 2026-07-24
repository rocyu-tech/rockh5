// Privacy Policy page (placeholder — must be reviewed by legal + DPO).

export default function PrivacyPage() {
  return (
    <div className="min-h-screen bg-[#0a0a1a] text-white">
      <div className="max-w-2xl mx-auto px-4 py-12">
        <h1 className="text-2xl font-bold mb-6">Privacy Policy</h1>
        <div className="prose prose-invert max-w-none">
          <p className="text-sm text-[#8892b0] mb-4">
            Last updated: {new Date().toLocaleDateString()}
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">1. Data We Collect</h2>
          <p className="text-sm leading-relaxed mb-4">
            We collect the following personal data when you register and use the Service:
          </p>
          <ul className="text-sm leading-relaxed mb-4 list-disc pl-6">
            <li><strong>Account data:</strong> email, nickname, hashed password, preferred language, timezone.</li>
            <li><strong>Identity verification (KYC):</strong> full name, date of birth, government-issued ID number, ID document scans. Required for real-money transactions.</li>
            <li><strong>Financial data:</strong> deposit and withdrawal history, wallet balances, payment method details (bank account number / e-wallet address / card last 4 digits).</li>
            <li><strong>Usage data:</strong> IP address, device type, browser, login timestamps, game play history, bet history.</li>
            <li><strong>Communications:</strong> support tickets, customer service chat logs.</li>
          </ul>

          <h2 className="text-lg font-semibold mt-6 mb-3">2. How We Use Your Data</h2>
          <ul className="text-sm leading-relaxed mb-4 list-disc pl-6">
            <li>To provide and improve the Service.</li>
            <li>To verify your identity (KYC) and prevent fraud.</li>
            <li>To process deposits and withdrawals.</li>
            <li>To comply with regulatory obligations (AML, CTF).</li>
            <li>To communicate with you about your account and the Service.</li>
          </ul>

          <h2 className="text-lg font-semibold mt-6 mb-3">3. Data Retention</h2>
          <p className="text-sm leading-relaxed mb-4">
            We retain your data for as long as your account is active, plus the regulatory retention period (typically 5 years for KYC and transaction records). After that, we will delete or anonymize your data unless retention is required by law.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">4. Data Sharing</h2>
          <p className="text-sm leading-relaxed mb-4">
            We share your data with:
          </p>
          <ul className="text-sm leading-relaxed mb-4 list-disc pl-6">
            <li>Payment processors (to process deposits and withdrawals).</li>
            <li>Game providers (PG Soft, Jili) for vendor-game sessions.</li>
            <li>Regulatory authorities when required by law.</li>
            <li>Identity verification services (KYC).</li>
          </ul>
          <p className="text-sm leading-relaxed mb-4">
            We never sell your personal data.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">5. Your Rights</h2>
          <ul className="text-sm leading-relaxed mb-4 list-disc pl-6">
            <li>Access your personal data (request a copy).</li>
            <li>Correct inaccurate data.</li>
            <li>Delete your account (subject to regulatory retention).</li>
            <li>Withdraw consent for non-essential processing.</li>
            <li>Lodge a complaint with your local data protection authority.</li>
          </ul>

          <h2 className="text-lg font-semibold mt-6 mb-3">6. Security</h2>
          <p className="text-sm leading-relaxed mb-4">
            We use AES-GCM encryption for sensitive data at rest (KYC documents, payment credentials), TLS 1.3 for data in transit, and httpOnly cookies for session tokens. Access to personal data is restricted to authorized personnel with audit logging.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">7. Cookies</h2>
          <p className="text-sm leading-relaxed mb-4">
            We use httpOnly cookies for authentication (essential). We do not use third-party tracking cookies without your consent.
          </p>

          <h2 className="text-lg font-semibold mt-6 mb-3">8. Contact</h2>
          <p className="text-sm leading-relaxed mb-4">
            For privacy questions or to exercise your rights, contact privacy@rockgame.example.
          </p>

          <p className="text-xs text-[#8892b0] mt-8 p-4 rounded bg-yellow-500/10 border border-yellow-500/30">
            ⚠️ This is a placeholder Privacy Policy. Before launch, this text MUST be reviewed by a qualified privacy lawyer / DPO to ensure compliance with GDPR, India DPDP Act, and other applicable data protection laws.
          </p>
        </div>
      </div>
    </div>
  );
}
