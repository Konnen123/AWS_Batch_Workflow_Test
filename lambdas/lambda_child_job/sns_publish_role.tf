data "aws_iam_policy_document" "lambda_sns_publish_policy" {
  statement {
    actions = [
      "sns:Publish",
    ]
    effect = "Allow"
    resources = [
      var.sns_child_job_finished_arn,
    ]
  }
}

resource "aws_iam_policy" "lambda_sns_publish" {
  name   = "${var.lambda_name}_sns_publish_policy"
  policy = data.aws_iam_policy_document.lambda_sns_publish_policy.json
}

resource "aws_iam_role_policy_attachment" "lambda_sns_publish_role_policy" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.lambda_sns_publish.arn
}