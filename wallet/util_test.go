package wallet

import "testing"

func TestParseVote(t *testing.T) {
	type args struct {
		vote string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   int
		want2   string
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:  "Success",
			args:  args{"+20|127.0.0.1"},
			want:  "+",
			want1: 20,
			want2: "127.0.0.1",
		},
		{
			name:  "Success",
			args:  args{"-20|127.0.0.1"},
			want:  "-",
			want1: 20,
			want2: "127.0.0.1",
		},
		{
			name:  "Success",
			args:  args{"+100|127.0.0.1"},
			want:  "+",
			want1: 100,
			want2: "127.0.0.1",
		},
		{
			name:  "Success",
			args:  args{"+20|127.0.0.1"},
			want:  "+",
			want1: 20,
			want2: "127.0.0.1",
		},
		{
			name:  "Decimal percentage: should give 0",
			args:  args{"+1.6|127.0.0.1"},
			want:  "+",
			want1: 0,
			want2: "127.0.0.1",
		},
		{
			name:    ">100 percentage",
			args:    args{"+101|127.0.0.1"},
			want:    "",
			want1:   0,
			want2:   "",
			wantErr: true,
		},
		{
			name:    ">100 percentage",
			args:    args{"+101|127.0.0.1"},
			want:    "",
			want1:   0,
			want2:   "",
			wantErr: true,
		},
		{
			name:    "Invalid separater",
			args:    args{"+20/127.0.0.1"},
			want:    "",
			want1:   0,
			want2:   "",
			wantErr: true,
		},
		{
			name:    "No vote percentage",
			args:    args{"+|127.0.0.1"},
			want:    "",
			want1:   0,
			want2:   "",
			wantErr: true,
		},
		{
			name:    "No address provided",
			args:    args{"+|"},
			want:    "",
			want1:   0,
			want2:   "",
			wantErr: true,
		},
		{
			name:    "No sign provided",
			args:    args{"30|127.0.0.1"},
			want:    "",
			want1:   0,
			want2:   "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, err := ParseVote(tt.args.vote)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVote() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseVote() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("ParseVote() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("ParseVote() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}
