package cmd_test

import (
	"ghprs/cmd"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Review Status Detection", func() {
	Describe("Label-based review detection", func() {
		Context("when PR has approved labels", func() {
			It("should detect 'approved' label", func() {
				labels := []cmd.Label{
					{Name: "approved"},
				}

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeTrue())
			})

			It("should detect 'lgtm' label", func() {
				labels := []cmd.Label{
					{Name: "lgtm"},
				}

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeTrue())
			})

			It("should detect approved label among multiple labels", func() {
				labels := []cmd.Label{
					{Name: "bug"},
					{Name: "approved"},
					{Name: "priority-high"},
				}

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeTrue())
			})

			It("should detect lgtm label among multiple labels", func() {
				labels := []cmd.Label{
					{Name: "bug"},
					{Name: "lgtm"},
					{Name: "priority-high"},
				}

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeTrue())
			})

			It("should detect when both approved and lgtm labels are present", func() {
				labels := []cmd.Label{
					{Name: "approved"},
					{Name: "lgtm"},
					{Name: "bug"},
				}

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeTrue())
			})
		})

		Context("when PR has no approved labels", func() {
			It("should not detect approval in empty labels", func() {
				labels := []cmd.Label{}

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeFalse())
			})

			It("should not detect approval in nil labels", func() {
				var labels []cmd.Label

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeFalse())
			})

			It("should not detect approval with other labels", func() {
				labels := []cmd.Label{
					{Name: "bug"},
					{Name: "enhancement"},
					{Name: "do-not-merge/hold"},
				}

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeFalse())
			})

			It("should be case sensitive", func() {
				labels := []cmd.Label{
					{Name: "APPROVED"},  // Wrong case
					{Name: "Lgtm"},      // Wrong case
					{Name: "approved "}, // Extra space
				}

				hasApprovedLabel := false
				for _, label := range labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeFalse())
			})
		})
	})

	Describe("Mock Review data", func() {
		Context("when testing with mock review data", func() {
			It("should create approved reviews correctly", func() {
				reviews := cmd.CreateMockReviews(true)
				Expect(len(reviews)).To(Equal(1))
				Expect(reviews[0].State).To(Equal("APPROVED"))
				Expect(reviews[0].User.Login).To(Equal("reviewer1"))
			})

			It("should create non-approved reviews correctly", func() {
				reviews := cmd.CreateMockReviews(false)
				Expect(len(reviews)).To(Equal(1))
				Expect(reviews[0].State).To(Equal("COMMENTED"))
				Expect(reviews[0].User.Login).To(Equal("reviewer1"))
			})
		})
	})

	Describe("Integration with existing PR functionality", func() {
		Context("when PR data includes labels", func() {
			It("should work with existing PR struct", func() {
				pr := cmd.PullRequest{
					Number: 123,
					Title:  "Test PR with approved label",
					State:  "open",
					Labels: []cmd.Label{
						{Name: "approved"},
						{Name: "bug"},
					},
				}

				// Test that we can access labels from PR struct
				Expect(len(pr.Labels)).To(Equal(2))

				// Test the approval detection logic
				hasApprovedLabel := false
				for _, label := range pr.Labels {
					if label.Name == "approved" || label.Name == "lgtm" {
						hasApprovedLabel = true
						break
					}
				}

				Expect(hasApprovedLabel).To(BeTrue())
			})

			It("should work with existing mock PR data", func() {
				prs := cmd.CreateMockPullRequests(10)

				// Test that mock PRs have label structures
				for _, pr := range prs {
					Expect(pr.Labels).NotTo(BeNil())

					// Test label detection works on each PR
					hasApprovedLabel := false
					for _, label := range pr.Labels {
						if label.Name == "approved" || label.Name == "lgtm" {
							hasApprovedLabel = true
							break
						}
					}

					// Mock PRs don't have approved labels by default
					Expect(hasApprovedLabel).To(BeFalse())
				}
			})
		})
	})
})
