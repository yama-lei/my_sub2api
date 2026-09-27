export default {
  modelTrace: {
    title: 'Model Authenticity Check',
    intro:
      'Powered by ModelTrace fingerprint attribution: sends 3 "long integer first-instinct" probes to the selected model and compares the number fingerprints against the embedded reference bank to decide whether the responses actually come from the claimed model (detection of watered-down or spoofed models). The check makes several upstream calls and usually takes 1-5 minutes.',
    selectAccount: 'Select account (key)',
    selectAccountPlaceholder: 'Choose the account used to send the probes',
    selectAccountHint: 'Only Anthropic / OpenAI OAuth and API Key accounts are listed; probes are sent directly with that account credential.',
    selectModel: 'Select model to check',
    selectModelPlaceholder: 'Choose the model name to verify',
    selectModelHint: 'Only models covered by the fingerprint bank are supported (16 GPT / Claude models).',
    run: 'Start check',
    rerun: 'Run again',
    runningHint: 'Check in progress, please keep this dialog open…',
    costHint: 'The check consumes upstream quota of the selected account (about 3-6 calls)',
    abortedOnClose: 'Check cancelled',
    claimedModel: 'Claimed model',
    predictedModel: 'Fingerprint verdict',
    predictionProbability: 'Verdict probability',
    claimedProbability: 'Claimed model probability',
    familyProbabilities: 'Family probabilities',
    topModels: 'Best matching models',
    usedOutputs: 'Based on {count} valid responses',
    collectionDetails: 'Collection details',
    disclaimer: 'Results are for reference only and are not decisive evidence; unknown models are attributed to the closest existing candidate.',
    verdict: {
      match: 'Verdict: consistent — responses match the claimed model',
      family_mismatch: 'Likely watered down / downgraded — same family but a different version',
      mismatch: 'Likely spoofed — responses actually come from another model family'
    },
    confidence: {
      high: 'high confidence',
      medium: 'medium confidence',
      low: 'low confidence'
    },
    progress: {
      started: 'Checking {model} …',
      done: 'Check finished',
      failed: 'Check failed',
      aborted: 'Cancelled'
    },
    table: {
      model: 'Model',
      probability: 'Probability',
      similarity: 'Fingerprint similarity',
      claimedTag: 'claimed'
    }
  }
}
