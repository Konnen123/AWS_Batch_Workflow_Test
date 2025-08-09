resource "aws_lambda_permission" "allow_sns" {
  statement_id  = "AllowExecutionFromSNS"
  action        = "lambda:InvokeFunction"
  function_name = var.lambda_name
  principal     = "sns.amazonaws.com"
  source_arn    = var.sns_child_job_finished_arn
}
resource "aws_sns_topic_subscription" "lambda_sub" {
  topic_arn = var.sns_child_job_finished_arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.create_batch_jobs.arn
}