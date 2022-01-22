GIT_REMOTE_URL=$(git remote get-url origin)
GIT_AUTHOR_EMAIL=$(git log -1 --pretty=format:'%ae')
GIT_TAG=$(git describe --tags --dirty)
GIT_HEAD=$(git rev-parse HEAD)
BLUE_RELEASE_TAG=blue-$GIT_TAG
BLUE_RELEASE_TAR=target/blue-release-$GIT_TAG.tar.gz
BLUE_EXEC=bluectl

if [[ ! -v BLUE_PGP_KEY ]]; then
    echo "BLUE_PGP_KEY is not set. See bluectl release create -h for help."
	exit 1;
fi

if [[ ! -v BLUE_PGP_KEYRING ]]; then
    echo "BLUE_PGP_KEYRING is not set. See bluectl release create -h for help."
	exit 1;
fi

blue_release_dist() {
	GIT_LOG=$(git log --pretty=format:"%h: %s" $GIT_LOG_RANGE)
	printf "\n$GIT_LOG\n";

	$BLUE_EXEC release create -d git-remote-url=$GIT_REMOTE_URL -d git-author-email=$GIT_AUTHOR_EMAIL -d git-tag=$GIT_TAG -d git-head=$GIT_HEAD -d git-log="$GIT_LOG" -k $BLUE_PGP_KEY -r $BLUE_PGP_KEYRING $BLUE_RELEASE_TAG $BLUE_RELEASE_TAR
}

# check if HEAD is tagged; if not, use annotate with range between latest tag and HEAD
git describe --contains 2>&1 1> /dev/null;
if [ $? -ne 0 ];
then
	GIT_LOG_RANGE="$(git tag -l --sort=-version:refname | head -1)...HEAD";
	printf "using git range between latest tag and latest tag + added commits: $GIT_LOG_RANGE:";
	blue_release_dist;
else
	GIT_LOG_RANGE="$(git tag -l --sort=-version:refname | head -2 | xargs | sed 's/ /.../g')";
	printf "using git range between latest tags: $GIT_LOG_RANGE:";
	blue_release_dist;
fi
