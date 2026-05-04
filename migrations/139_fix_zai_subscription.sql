-- Migration 139: Fix Z.AI subscription data to reflect free extension
-- User had $45/quarterly subscription (Feb-Apr 2026). Z.AI removed legacy unlimited account.
-- Compensation: 2 months free (May 2 - July 2 2026) with new plan limits.
-- $15/mo recurring DOES NOT EXIST. No active paid subscription.

UPDATE subscription_history SET
  cost_usd = 0,
  period_type = 'free',
  ended_at = '2026-07-02 00:00:00-04',
  notes = 'Z.AI Pro FREE extension (May 2 - July 2 2026). Compensation for removing unlimited legacy account. New limits: 400 req/5hr, 2000 req/week. GLM-5 model. Renewal after July 2 would be full paid plan at much higher cost.'
WHERE model_id = 'glm-5' AND provider = 'zhipu';

-- Fix project_costs to match
UPDATE project_costs SET
  description = 'Z.AI Pro (GLM-5) - FREE extension May-Jul 2026 (was $45/quarterly, now free)',
  amount_usd = 0
WHERE category = 'subscription' AND description LIKE '%Z.AI%';

-- Archive the bogus test internet entry
UPDATE project_costs SET archived_at = now()
WHERE description = 'Test internet';
