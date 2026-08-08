class Solution {
public:
    int f(int n , vector<int> &dp){
        if(n==0 || n==1) return 1;
        if(dp[n]!=-1) return dp[n];
        int sum=0;
        for(int i=1;i<=n;i++ ){//i is a root 
            int r=f(n-i,dp);//no. unique bst of right 

            int l=f(i-1,dp);//no. unique bst of left 

            sum+=r*l;// no. of unqiue bst for  partcular node  is no. unique bst of right multiplied by no. unique bst of left 

          //summing becuase we are chekcing by making each node as root and then calcuting hwo many unisuq it forms 

        }
        return dp[n]=sum;
    }
    int numTrees(int n) {
       vector<int> dp(n+1,-1);
       dp[0]=1,dp[1]=1;
       return f(n,dp);


    }
};

#Explaination 
