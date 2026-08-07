// 1 - Brute Force Solution
/**
 * Definition for a binary tree node.
 * struct TreeNode {
 *     int val;
 *     TreeNode *left;
 *     TreeNode *right;
 *     TreeNode(int x) : val(x), left(NULL), right(NULL) {}
 * };
 */

class Solution {
public:
    int preorder(TreeNode* node, vector<TreeNode*>& ans,int val){
        if (node == NULL) return 0;
        ans.push_back(node);
        if (node->val == val) return -1;
        if (preorder(node->left,ans,val)==-1){
            return -1;
        }
        if (preorder(node->right,ans,val) == -1){
            return -1;
        }
        ans.pop_back();
        return 0;
    }
    TreeNode* lowestCommonAncestor(TreeNode* root, TreeNode* p, TreeNode* q) {
        vector<TreeNode*> l;
        vector<TreeNode*> r;
        preorder(root,l,p->val);
        preorder(root,r,q->val);
        for (int i =l.size() - 1 ; i>=0;i--){
            for (int j =r.size()-1 ; j>=0;j--){
                if (l[i]->val == r[j]->val){
                    return l[i];
                }
            }
        }  
        return root;    
    }
};
