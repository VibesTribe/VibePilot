-- Create pg_notify triggers for the vp_changes channel.
-- The governor listens on vp_changes to route domain events.
-- Without these triggers, research suggestions, reports, tasks, etc.
-- are inserted but never processed by the governor.

CREATE TRIGGER notify_research_suggestions AFTER INSERT OR UPDATE ON research_suggestions
  FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

CREATE TRIGGER notify_research_reports AFTER INSERT OR UPDATE ON research_reports
  FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

CREATE TRIGGER notify_research_report_items AFTER INSERT OR UPDATE ON research_report_items
  FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

CREATE TRIGGER notify_project_todos AFTER INSERT OR UPDATE ON project_todos
  FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

CREATE TRIGGER notify_review_items AFTER INSERT OR UPDATE ON review_items
  FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

CREATE TRIGGER notify_model_catalog AFTER INSERT OR UPDATE ON model_catalog
  FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

CREATE TRIGGER notify_plans AFTER INSERT OR UPDATE ON plans
  FOR EACH ROW EXECUTE FUNCTION vp_notify_change();

CREATE TRIGGER notify_task_runs AFTER INSERT OR UPDATE ON task_runs
  FOR EACH ROW EXECUTE FUNCTION vp_notify_change();
