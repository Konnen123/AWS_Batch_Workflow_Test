module "s3_bucket" {
  source = "./s3"
  bucket_name = "test-image-bucket-e7a7e1da-a8c0-4755-963b-0c2c53501258"
}

module "lambda_create_batch_jobs" {
  source = "./lambdas/lambda_create_batch_jobs"
  lambda_name = "create-batch-jobs"
  s3_arn = module.s3_bucket.arn
  s3_name = module.s3_bucket.name
  sns_create_job_arn = module.sns_create_job.arn
  dynamodb_arn = module.dynamodb_table.arn
  dynamodb_table_name = module.dynamodb_table.name
}

module "lambda_child_job" {
  source = "./lambdas/lambda_child_job"
  lambda_name = "child-job"
  s3_name = module.s3_bucket.name
  sns_create_job_arn = module.sns_create_job.arn
  s3_arn = module.s3_bucket.arn
  dynamodb_arn = module.dynamodb_table.arn
  dynamodb_table_name = module.dynamodb_table.name
  sns_child_job_finished_arn = module.sns_child_job_finished.arn
}

module "lambda_merge_zip_files" {
  source = "./lambdas/lambda_merge_zip_files"
  dynamodb_arn = module.dynamodb_table.arn
  dynamodb_table_name = module.dynamodb_table.name
  lambda_name = "merge-zip-files"
  s3_arn = module.s3_bucket.arn
  s3_name = module.s3_bucket.name
  sns_child_job_finished_arn = module.sns_child_job_finished.arn
}

module "sns_create_job" {
  source = "./sns/sns-create-job"
  topic_name = "sns-create-job"
}

module "sns_child_job_finished"{
  source = "./sns/sns-child-job-finished"
  topic_name = "sns-child-job-finished"
}

module "dynamodb_table" {
  source = "./dynamodb"
  dynamodb_table_name = "test-dynamodb-table-3b2c4f8a-1d5e-4c0b-9f6d-7c8e1f2a3b4c"
}