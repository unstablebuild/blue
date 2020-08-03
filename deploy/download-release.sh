#!/usr/bin/env bash
#
# This is an adaptation of code I wrote to download a private binary from GitHub. Such...pain.
# Why can't GitHub just offer a standardized URL you can download a release binary from and attach 
# your Github Personal Access Token as a header? 
#
# Since this code is an adaptation it hasn't been directly tested, but the code it was adapted from works
# and hopefully you can get the missing piece you're after by looking here.
#
# Adapted from Gist:
# https://gist.github.com/josh-padnick/fdae42c07e648c798fc27dec2367da21
set -e

# Parse CLI args
if [[ -z $1 && -z $2 ]]; then
	echo "usage: $0 <release-tag> <amd64|arm64>"
    exit 1
fi

readonly github_oauth_token="$GITHUB_TOKEN"
readonly git_tag="$1"
readonly arch="$2"
readonly github_repo_owner="ernestrc"
readonly github_repo_name="blue"
readonly release_asset_filenames=(\
	"gps-server-$git_tag-linux-$arch.tar.gz" \
	"gps-producer-$git_tag-linux-$arch.tar.gz" \
)

# Get the "github tag id" of this release
github_tag_id=$(curl --silent --show-error \
                     --header "Authorization: token $github_oauth_token" \
                     --request GET \
                     "https://api.github.com/repos/$github_repo_owner/$github_repo_name/releases" \
                     | jq --raw-output ".[] | select(.tag_name==\"$git_tag\").id")

for filename in ${release_asset_filenames[@]}; do
  # Get the download URL of our desired asset
  download_url=$(curl --silent --show-error \
                      --header "Authorization: token $github_oauth_token" \
                      --header "Accept: application/vnd.github.v3.raw" \
                      --location \
                      --request GET \
                      "https://api.github.com/repos/$github_repo_owner/$github_repo_name/releases/$github_tag_id" \
                      | jq --raw-output ".assets[] | select(.name==\"$filename\").url")
  
  # Get GitHub's S3 redirect URL
  # Why not just curl's built-in "--location" option to auto-redirect? Because curl then wants to include all the original
  # headers we added for the GitHub request, which makes AWS complain that we're trying strange things to authenticate.
  redirect_url=$(curl --silent --show-error \
            --header "Authorization: token $github_oauth_token" \
            --header "Accept: application/octet-stream" \
            --request GET \
            --write-out "%{redirect_url}" \
            "$download_url")
  
  # Finally download the actual binary
  curl --silent --show-error \
            --header "Accept: application/octet-stream" \
            --output "$filename" \
            --request GET \
            "$redirect_url"
done
