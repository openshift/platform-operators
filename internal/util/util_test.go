package util

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/openshift/api/platform/v1alpha1"
	platformtypes "github.com/openshift/platform-operators/api/v1alpha1"
)

func Test_inspectPlatformOperator(t *testing.T) {
	type args struct {
		po platformv1alpha1.PlatformOperator
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "HappyPath",
			args: args{
				po: platformv1alpha1.PlatformOperator{
					Status: platformv1alpha1.PlatformOperatorStatus{
						Conditions: []metav1.Condition{
							{
								Type:   platformtypes.TypeApplied,
								Status: metav1.ConditionTrue,
								Reason: platformtypes.ReasonApplySuccessful,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "NilConditions",
			args: args{
				po: platformv1alpha1.PlatformOperator{
					Status: platformv1alpha1.PlatformOperatorStatus{
						Conditions: []metav1.Condition{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "InstallFailed",
			args: args{
				po: platformv1alpha1.PlatformOperator{
					Status: platformv1alpha1.PlatformOperatorStatus{
						Conditions: []metav1.Condition{
							{
								Type:   platformtypes.TypeApplied,
								Status: metav1.ConditionFalse,
								Reason: platformtypes.ReasonApplyFailed,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "SourceFailed",
			args: args{
				po: platformv1alpha1.PlatformOperator{
					Status: platformv1alpha1.PlatformOperatorStatus{
						Conditions: []metav1.Condition{
							{
								Type:   platformtypes.TypeApplied,
								Status: metav1.ConditionFalse,
								Reason: platformtypes.ReasonSourceFailed,
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "MissingAppliedCondition",
			args: args{
				po: platformv1alpha1.PlatformOperator{
					Status: platformv1alpha1.PlatformOperatorStatus{
						Conditions: []metav1.Condition{
							{
								Type:   "stub",
								Status: metav1.ConditionTrue,
								Reason: platformtypes.ReasonSourceFailed,
							},
						},
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := inspectPlatformOperator(tt.args.po); (err != nil) != tt.wantErr {
				t.Errorf("inspectPlatformOperator() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
