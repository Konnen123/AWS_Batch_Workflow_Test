resource "null_resource" "build_lambda" {
  triggers = {
    source_code_hash = base64sha256(filebase64(local.lambda_main_file_path))
  }

  provisioner "local-exec" {
    command     = "cd ${local.lambda_folder_path} && make zip_file"
    interpreter = ["/bin/bash", "-c"]
  }
}