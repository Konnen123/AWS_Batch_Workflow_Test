data "aws_iam_policy_document" "lambda_dynamodb_policy" {
  statement {
    actions = [
      "dynamodb:PutItem",
    ]
    effect = "Allow"
    resources = [
      var.dynamodb_arn,
    ]
  }
}

resource "aws_iam_policy" "lambda_dynamodb_publish" {
  name   = "${var.lambda_name}_dynamodb_policy"
  policy = data.aws_iam_policy_document.lambda_dynamodb_policy.json
}

resource "aws_iam_role_policy_attachment" "lambda_dynamodb_role_policy" {
  role       = aws_iam_role.lambda_role.name
  policy_arn = aws_iam_policy.lambda_dynamodb_publish.arn
}