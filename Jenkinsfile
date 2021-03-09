@Library('mdblp-library') _
def builderImage
pipeline {
    agent any
    stages {
        stage('Initialization') {
            steps {
                script {
                    utils.initPipeline()
                    if(env.GIT_COMMIT == null) {
                        // git commit id must be a 40 characters length string (lower case or digits)
                        env.GIT_COMMIT = "f".multiply(40)
                    }
                    builderImage = docker.build('go-build-image','-f ./Dockerfile.build .')
                }
            }
        }
        stage('Build ') {
            steps {
                script {
                    builderImage.inside("") {
                        withCredentials ([string(credentialsId: 'github-token', variable: 'GITHUB_TOKEN')]) {
                            sh 'git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"'
                            sh "$WORKSPACE/build.sh"
                            sh 'git config --global --unset url."https://${GITHUB_TOKEN}@github.com/".insteadOf'
                        }
                    }
                }
            }
        }
        stage('Test ') {
            steps {
                echo 'start mongo to serve as a testing db'
                sh 'docker network create hydrotest${BUILD_NUMBER} && docker run --rm -d --net=hydrotest${BUILD_NUMBER} --name=mongo4hydrotest${BUILD_NUMBER} mongo:4.2'
                script {
                    builderImage.inside("--net=hydrotest${BUILD_NUMBER}") {
                        withCredentials ([string(credentialsId: 'github-token', variable: 'GITHUB_TOKEN')]) {
                            sh 'git config --global url."https://${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"'
                            sh "TIDEPOOL_STORE_ADDRESSES=mongo4hydrotest${BUILD_NUMBER}:27017 TIDEPOOL_STORE_DATABASE=confirm_test $WORKSPACE/test.sh"
                            sh 'git config --global --unset url."https://${GITHUB_TOKEN}@github.com/".insteadOf'
                        }
                    }
                }
            }
            post {
                always {
                    sh 'docker stop mongo4hydrotest${BUILD_NUMBER} && docker network rm hydrotest${BUILD_NUMBER}'
                }
            }
        }
        stage('Package') {
            steps {
                withCredentials ([string(credentialsId: 'github-token', variable: 'GITHUB_TOKEN')]) {
                    sh "docker build --build-arg GITHUB_TOKEN=${GITHUB_TOKEN} -t docker.ci.diabeloop.eu/hydrophone:${GIT_COMMIT} ."
                }
            }
        }
        /*
        stage('Documentation') {
            steps {
                script {
                    builderImage.inside("") {
                        sh "./qa/buildDoc.sh"
                    }
                }
                
            }
        }*/
        stage('Publish') {
            when { branch "dblp" }
            steps {
                publish()
            }
        }
    }
}
