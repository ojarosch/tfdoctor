module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
}

module "branchy" {
  source = "git::https://github.com/example/mod.git?ref=main"
}

module "noref" {
  source = "git::https://github.com/example/noref.git"
}
