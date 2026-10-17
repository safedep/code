class UsersController < ApplicationController
  before_action :authenticate

  def show
    user = User.find(params[:id])
    render json: user.profile
  end

  private

  def authenticate
    Rails.logger.info('auth')
  end
end
